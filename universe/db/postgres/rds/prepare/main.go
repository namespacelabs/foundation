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
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
	inclustertool "namespacelabs.dev/foundation/universe/db/postgres/incluster/configure"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/toolcommon"
	"namespacelabs.dev/foundation/universe/db/postgres/rds"
	"namespacelabs.dev/foundation/universe/db/postgres/rds/internal"
)

const (
	postgresType = "rds"

	creds = "namespacelabs.dev/foundation/universe/db/postgres/internal/gencreds"

	self    = "namespacelabs.dev/foundation/universe/db/postgres/rds/prepare"
	rdsInit = "namespacelabs.dev/foundation/universe/db/postgres/rds/internal/init"
	rdsNode = "namespacelabs.dev/foundation/universe/db/postgres/rds"

	inclusterInit   = "namespacelabs.dev/foundation/universe/db/postgres/internal/init"
	inclusterServer = "namespacelabs.dev/foundation/universe/db/postgres/server"
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

func (provisionHook) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	dbs := map[string]*rds.Database{}
	owners := map[string][]string{}
	if err := allocations.Visit(req.Focus.Server.Allocation, rdsNode, &rds.Database{},
		func(alloc *schema.Allocation_Instance, instantiate *schema.Instantiate, db *rds.Database) error {
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
		return applyIncluster(ctx, req, dbs, owners, out)
	}

	return applyRds(ctx, req, dbs, out)
}

func (provisionHook) Delete(ctx context.Context, req configure.StackRequest, out *configure.DeleteOutput) error {
	if useIncluster(req.Env) {
		return inclustertool.Delete(ctx, req, out)
	}

	// TODO
	return nil
}

func applyIncluster(ctx context.Context, req configure.StackRequest, dbs map[string]*rds.Database, owners map[string][]string, out *configure.ApplyOutput) error {
	inclusterDbs := map[string]*incluster.Database{}

	for name, db := range dbs {
		inclusterDb := &incluster.Database{
			Name:       db.Name,
			SchemaFile: db.SchemaFile,
		}
		inclusterDbs[name] = inclusterDb
	}

	return inclustertool.Apply(ctx, req, inclusterDbs, owners, out)
}

func applyRds(ctx context.Context, req configure.StackRequest, dbs map[string]*rds.Database, out *configure.ApplyOutput) error {
	systemInfo := &kubedef.SystemInfo{}
	if err := req.UnpackInput(systemInfo); err != nil {
		return err
	}

	eksDetails := &eks.EKSServerDetails{}
	if err := req.UnpackInput(eksDetails); err != nil {
		return err
	}

	var orderedDbs []*rds.Database
	for _, db := range dbs {
		orderedDbs = append(orderedDbs, db)
	}

	sort.Slice(orderedDbs, func(i, j int) bool {
		return strings.Compare(orderedDbs[i].Name, orderedDbs[j].Name) < 0
	})

	// TODO improve robustness - configurable?
	if len(systemInfo.Regions) != 1 {
		return fmt.Errorf("Unable to infer region.")
	}
	region := systemInfo.Regions[0]

	clusterArns := make([]string, len(orderedDbs))
	dbArns := make([]string, len(orderedDbs))
	for k, db := range orderedDbs {
		// TODO all accounts? Really?
		id := internal.ClusterIdentifier(db.Name)
		clusterArns[k] = fmt.Sprintf("arn:aws:rds:%s:*:cluster:%s", region, id)
		dbArns[k] = fmt.Sprintf("arn:aws:rds:%s:*:db:%s*", region, id)
	}

	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference_policies_examples_rds_region.html
	policy := fniam.PolicyDocument{
		Version: "2012-10-17",
		Statement: []fniam.StatementEntry{
			{
				Effect:   "Allow",
				Action:   []string{"rds:*"},
				Resource: clusterArns,
			},
			{
				Effect:   "Allow",
				Action:   []string{"rds:*"},
				Resource: dbArns,
			},
		},
	}

	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return fnerrors.InternalError("failed to serialize policy: %w", err)
	}

	associate := &fniam.OpAssociatePolicy{
		RoleName:   eksDetails.ComputedIamRoleName,
		PolicyName: "fn-universe-db-postgres-rds-access",
		PolicyJson: string(policyBytes),
	}

	out.Invocations = append(out.Invocations, defs.Static("RDS Postgres Access IAM Policy", associate))

	baseDbs := map[string]*postgres.Database{}
	for name, db := range dbs {
		baseDbs[name] = &postgres.Database{
			Name:       db.Name,
			SchemaFile: db.SchemaFile,
		}
	}

	col, err := secrets.Collect(req.Focus.Server)
	if err != nil {
		return err
	}

	initArgs := []string{}
	// TODO: creds should be definable per db instance #217
	var credsSecret *secrets.SecretDevMap_SecretSpec
	for _, secret := range col.SecretsOf(creds) {
		if secret.Name == "postgres-password-file" {
			initArgs = append(initArgs, fmt.Sprintf("--postgres_password_file=%s", secret.FromPath))
			break
		}
	}
	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/aws/client") {
		if secret.Name == "aws_credentials_file" {
			initArgs = append(initArgs, fmt.Sprintf("--aws_credentials_file=%s", secret.FromPath))
			break
		}
	}

	if credsSecret != nil {
		initArgs = append(initArgs, fmt.Sprintf("--postgres_password_file=%s", credsSecret.FromPath))
	}

	return toolcommon.ApplyForInit(ctx, req, baseDbs, postgresType, rdsInit, initArgs, out)
}
