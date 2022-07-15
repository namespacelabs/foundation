// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package configure

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/toolcommon"
)

const postgresType = "incluster"

func internalEndpoint(s *schema.Stack) *schema.Endpoint {
	for _, e := range s.Endpoint {
		if e.ServiceName == "postgres" && e.ServerOwner == "namespacelabs.dev/foundation/universe/db/postgres/server" {
			return e
		}
	}

	return nil
}

func Apply(ctx context.Context, r configure.StackRequest, dbs map[string]*incluster.Database, owners map[string][]string, out *configure.ApplyOutput) error {
	initArgs := []string{}

	col, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	// TODO: creds should be definable per db instance #217
	var credsSecret *secrets.SecretDevMap_SecretSpec
	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/db/postgres/internal/gencreds") {
		if secret.Name == "postgres-password-file" {
			credsSecret = secret
		}
	}

	if credsSecret != nil {
		initArgs = append(initArgs, fmt.Sprintf("--postgres_password_file=%s", credsSecret.FromPath))
	}

	endpoint := internalEndpoint(r.Stack)

	value, err := json.Marshal(endpoint)
	if err != nil {
		return err
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			Args: []string{fmt.Sprintf("--%s=%s", incluster.EndpointFlag, value)},
			// XXX remove when backwards compat no longer necessary.
			ArgTuple: []*kubedef.ContainerExtension_ArgTuple{{
				Name:  incluster.EndpointFlag,
				Value: string(value),
			}},
		}})

	var creds *postgres.InstantiatedDatabase_Credentials
	if credsSecret != nil {
		creds = &postgres.InstantiatedDatabase_Credentials{
			SecretName:         credsSecret.Name,
			SecretMountPath:    credsSecret.FromPath,
			SecretResourceName: credsSecret.ResourceName,
		}
	}

	endpointedDbs := map[string]*postgres.Database{}
	for name, db := range dbs {
		endpointedDbs[name] = &postgres.Database{
			Name:       db.Name,
			SchemaFile: db.SchemaFile,
			HostedAt: &postgres.Endpoint{
				Address: endpoint.AllocatedName,
				Port:    uint32(endpoint.Port.ContainerPort),
			},
		}
	}

	computed := &postgres.InstantiatedDatabases{}
	ownedDbs := map[string][]*postgres.Database{}
	for name, db := range endpointedDbs {
		for _, owner := range owners[name] {
			ownedDbs[owner] = append(ownedDbs[owner], db)
		}
	}
	for owner, databases := range ownedDbs {
		computed.Instantiated = append(computed.Instantiated, &postgres.InstantiatedDatabase{
			PackageName: owner,
			Credentials: creds,
			Database:    databases,
		})
	}

	serializedComputed, err := anypb.New(computed)
	if err != nil {
		return err
	}

	out.Computed = append(out.Computed, &schema.ComputedConfiguration{
		Owner: r.PackageOwner(),
		Impl:  serializedComputed,
	})

	return toolcommon.Apply(ctx, r, endpointedDbs, postgresType, initArgs, out)
}

func Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return toolcommon.Delete(r, postgresType, out)
}
