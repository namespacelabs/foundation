// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/providers/aws/eks"
	fniam "namespacelabs.dev/foundation/providers/aws/iam"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/allocations"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/toolcommon"
	"namespacelabs.dev/foundation/universe/db/postgres/rds"
	"namespacelabs.dev/foundation/universe/db/postgres/rds/internal"
)

const (
	postgresType = "rds"

	creds = "namespacelabs.dev/foundation/universe/db/postgres/internal/gencreds"

	self    = "namespacelabs.dev/foundation/universe/db/postgres/rds/prepare"
	rdsInit = "namespacelabs.dev/foundation/universe/db/postgres/rds/init"
	rdsNode = "namespacelabs.dev/foundation/universe/db/postgres/rds"

	inclusterNode   = "namespacelabs.dev/foundation/universe/db/postgres/incluster"
	inclusterInit   = "namespacelabs.dev/foundation/universe/db/postgres/internal/init"
	inclusterServer = "namespacelabs.dev/foundation/universe/db/postgres/rds/testing/server"
)

func main() {
	if err := configure.RunServer(context.Background(), func(sr grpc.ServiceRegistrar) {
		h := configure.NewHandlers()
		h.Any().HandleStack(provisionHook{})

		protocol.RegisterPrepareServiceServer(sr, prepareHook{})
		protocol.RegisterInvocationServiceServer(sr, h.ServiceHandler())
	}); err != nil {
		log.Fatal(err)
	}
}

func useIncluster(env *schema.Environment) bool {
	return env.GetPurpose() == schema.Environment_DEVELOPMENT || env.GetPurpose() == schema.Environment_TESTING
}

type prepareHook struct{}

func (prepareHook) Prepare(ctx context.Context, req *protocol.PrepareRequest) (*protocol.PrepareResponse, error) {
	resp := &protocol.PrepareResponse{
		PreparedProvisionPlan: &protocol.PreparedProvisionPlan{
			Provisioning: []*schema.Invocation{
				{Binary: self}, // Call me back.
			},
		},
	}

	// In development or testing, use incluster Postgres.
	if useIncluster(req.Env) {
		resp.PreparedProvisionPlan.Init = append(resp.PreparedProvisionPlan.Init, &schema.SidecarContainer{Binary: inclusterInit})
		resp.PreparedProvisionPlan.DeclaredStack = append(resp.PreparedProvisionPlan.DeclaredStack, inclusterServer)
	} else {
		resp.PreparedProvisionPlan.Init = append(resp.PreparedProvisionPlan.Init, &schema.SidecarContainer{Binary: rdsInit})
	}

	return resp, nil
}

type provisionHook struct{}

func (provisionHook) Apply(_ context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	dbs := map[string]*rds.Database{}
	owners := map[string][]string{}
	if err := allocations.Visit(req.Focus.Server.Allocation, rdsNode, &rds.Database{},
		func(alloc *schema.Allocation_Instance, _ *schema.Instantiate, db *rds.Database) error {
			if existing, ok := dbs[db.Name]; ok {
				if !proto.Equal(existing, db) {
					return fnerrors.UserError(nil, "%s: database definition for %q is incompatible with %s", alloc.InstanceOwner, db.Name, strings.Join(owners[db.Name], ","))
				}
			} else {
				dbs[db.Name] = db
			}

			owners[db.Name] = append(owners[db.Name], alloc.InstanceOwner)
			return nil
		}); err != nil {
		return err
	}

	if useIncluster(req.Env) {
		return applyIncluster(req, dbs, owners, out)
	}

	return applyRds(req, dbs, out)
}

func (provisionHook) Delete(_ context.Context, req configure.StackRequest, out *configure.DeleteOutput) error {
	if useIncluster(req.Env) {
		return toolcommon.Delete(req, postgresType, out)
	}

	// TODO
	return nil
}

func internalEndpoint(s *schema.Stack) *schema.Endpoint {
	for _, e := range s.Endpoint {
		if e.ServiceName == "postgres" && e.ServerOwner == inclusterServer {
			return e
		}
	}

	return nil
}

func applyIncluster(req configure.StackRequest, dbs map[string]*rds.Database, owners map[string][]string, out *configure.ApplyOutput) error {
	endpoint := internalEndpoint(req.Stack)

	value, err := json.Marshal(endpoint)
	if err != nil {
		return err
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			Args: []string{fmt.Sprintf("--%s=%s", rds.InclusterEndpointFlag, value)},
			// XXX remove when backwards compat no longer necessary.
			ArgTuple: []*kubedef.ContainerExtension_ArgTuple{{
				Name:  rds.InclusterEndpointFlag,
				Value: string(value),
			}},
		}})

	col, err := secrets.Collect(req.Focus.Server)
	if err != nil {
		return err
	}

	// TODO: creds should be definable per db instance #217
	var credsSecret *secrets.SecretDevMap_SecretSpec
	for _, secret := range col.SecretsOf(creds) {
		if secret.Name == "postgres-password-file" {
			credsSecret = secret
		}
	}

	endpointedDbs := map[string]*postgres.Database{}
	for name, db := range dbs {
		endpointedDbs[name] = &postgres.Database{
			Name:       db.Name,
			SchemaFile: db.SchemaFile,
			HostedAt: &postgres.Database_Endpoint{
				Address: endpoint.AllocatedName,
				Port:    uint32(endpoint.Port.ContainerPort),
			},
			Credentials: &postgres.Database_Credentials{
				Password: &postgres.Database_Credentials_Secret{
					FromPath: credsSecret.FromPath,
				},
			},
		}
	}

	return toolcommon.Apply(req, endpointedDbs, postgresType, out)
}

func applyRds(req configure.StackRequest, dbs map[string]*rds.Database, out *configure.ApplyOutput) error {
	var orderedDbs []*rds.Database
	for _, db := range dbs {
		orderedDbs = append(orderedDbs, db)
	}

	sort.Slice(orderedDbs, func(i, j int) bool {
		return strings.Compare(orderedDbs[i].Name, orderedDbs[j].Name) < 0
	})

	systemInfo := &kubedef.SystemInfo{}
	if err := req.UnpackInput(systemInfo); err != nil {
		return err
	}

	eksCluster := &eks.EKSCluster{}
	if err := req.UnpackInput(eksCluster); err != nil {
		return err
	}

	// TODO improve robustness - configurable?
	if len(systemInfo.Regions) != 1 {
		return fmt.Errorf("unable to infer region")
	}
	region := systemInfo.Regions[0]

	var rdsArns []string
	for _, db := range orderedDbs {
		// TODO all accounts? Really?
		id := internal.ClusterIdentifier(req.Env.Name, db.Name)
		rdsArns = append(rdsArns,
			fmt.Sprintf("arn:aws:rds:%s:*:cluster:%s", region, id),
			fmt.Sprintf("arn:aws:rds:%s:*:db:%s*", region, id),
			fmt.Sprintf("arn:aws:rds:%s:*:subgrp:*", region),
		)
	}

	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_examples_rds_region.html
	policy := fniam.PolicyDocument{
		Version: "2012-10-17",
		Statement: []fniam.StatementEntry{
			{
				Effect:   "Allow",
				Action:   []string{"rds:*"},
				Resource: rdsArns,
			},
			{
				Effect:   "Allow",
				Action:   []string{"ec2:Describe*"},
				Resource: []string{"*"}, // TODO, investigate why fmt.Sprintf("arn:aws:ec2:%s:*:subnet/*", region) doesn't work
			},
			{
				Effect: "Allow",
				Action: []string{
					"ec2:CreateSecurityGroup",
					"ec2:AuthorizeSecurityGroupIngress",
				}, // TODO should this be Namespace, not in the init?
				Resource: []string{
					fmt.Sprintf("arn:aws:ec2:%s:*:security-group/*", region),
					fmt.Sprintf("arn:aws:ec2:%s:*:vpc/%s", region, eksCluster.VpcId),
				}, // TODO all accounts? Really?
			},
		},
	}

	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return fnerrors.InternalError("failed to serialize policy: %w", err)
	}

	eksDetails := &eks.EKSServerDetails{}
	if err := req.UnpackInput(eksDetails); err != nil {
		return err
	}

	associate := &fniam.OpAssociatePolicy{
		RoleName:   eksDetails.ComputedIamRoleName,
		PolicyName: "fn-universe-db-postgres-rds-access",
		PolicyJson: string(policyBytes),
	}

	out.Invocations = append(out.Invocations, defs.Static("RDS Postgres Access IAM Policy", associate))

	col, err := secrets.Collect(req.Focus.Server)
	if err != nil {
		return err
	}

	initArgs := []string{
		fmt.Sprintf("--env_name=%s", req.Env.Name),
		fmt.Sprintf("--eks_vpc_id=%s", eksCluster.VpcId),
	}

	// TODO: creds should be definable per db instance #217
	var credsSecret *secrets.SecretDevMap_SecretSpec
	for _, secret := range col.SecretsOf(creds) {
		if secret.Name == "postgres-password-file" {
			credsSecret = secret
			break
		}
	}
	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/aws/client") {
		if secret.Name == "aws_credentials_file" {
			initArgs = append(initArgs, fmt.Sprintf("--aws_credentials_file=%s", secret.FromPath))
			break
		}
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			InitContainer: []*kubedef.ContainerExtension_InitContainer{{
				PackageName: rdsInit,
				Arg:         initArgs,
			}},
		}})

	baseDbs := map[string]*postgres.Database{}
	for name, db := range dbs {
		baseDbs[name] = &postgres.Database{
			Name:       db.Name,
			SchemaFile: db.SchemaFile,
			Credentials: &postgres.Database_Credentials{
				Password: &postgres.Database_Credentials_Secret{
					FromPath: credsSecret.FromPath,
				},
			},
		}
	}

	return toolcommon.ApplyForInit(req, baseDbs, postgresType, rdsInit, out)
}
