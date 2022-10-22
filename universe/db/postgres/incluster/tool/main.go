// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/allocations"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/toolcommon"
)

const postgresType = "incluster"

type tool struct{}

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	provisioning.Handle(h)
}

func internalEndpoint(s *schema.Stack) *schema.Endpoint {
	for _, e := range s.Endpoint {
		if e.ServiceName == "postgres" && e.ServerOwner == "namespacelabs.dev/foundation/universe/db/postgres/server" {
			return e
		}
	}

	return nil
}

func (tool) Apply(_ context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	dbs := map[string]*incluster.Database{}
	owners := map[string][]string{}
	if err := allocations.Visit(r.Focus.Server.Allocation, schema.PackageName(r.PackageOwner()), &incluster.Database{},
		func(alloc *schema.Allocation_Instance, _ *schema.Instantiate, db *incluster.Database) error {
			if existing, ok := dbs[db.GetName()]; ok {
				if !proto.Equal(existing, db) {
					return fnerrors.UserError(nil, "%s: database definition for %q is incompatible with %s", alloc.InstanceOwner, db.GetName(), strings.Join(owners[db.GetName()], ","))
				}
			} else {
				dbs[db.GetName()] = db
			}
			owners[db.GetName()] = append(owners[db.GetName()], alloc.InstanceOwner)
			return nil
		}); err != nil {
		return err
	}

	endpoint := internalEndpoint(r.Stack)

	value, err := json.Marshal(endpoint)
	if err != nil {
		return err
	}

	out.ServerExtensions = append(out.ServerExtensions, &schema.ServerExtension{
		ExtendContainer: []*schema.ContainerExtension{
			{Args: []string{fmt.Sprintf("--%s=%s", incluster.EndpointFlag, value)}},
		},
	})

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

	return toolcommon.Apply(r, endpointedDbs, postgresType, out)
}

func (tool) Delete(_ context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	return toolcommon.Delete(r, postgresType, out)
}
