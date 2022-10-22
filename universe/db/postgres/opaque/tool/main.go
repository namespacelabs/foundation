// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/allocations"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/toolcommon"
	"namespacelabs.dev/foundation/universe/db/postgres/opaque"
)

const postgresType = "opaque"

type tool struct{}

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	provisioning.Handle(h)
}

func collectDatabases(server *schema.Server, owner string) (map[string]*opaque.Database, error) {
	dbs := map[string]*opaque.Database{}
	owners := map[string][]string{}
	if err := allocations.Visit(server.Allocation, schema.PackageName(owner), &opaque.Database{},
		func(alloc *schema.Allocation_Instance, instantiate *schema.Instantiate, db *opaque.Database) error {
			if db.HostedAt == nil {
				return fnerrors.UserError(nil, "%s: database %q is missing an endpoint", alloc.InstanceOwner, db.GetName())
			}
			key := fmt.Sprintf("%s/%d/%s", db.HostedAt.GetAddress(), db.HostedAt.GetPort(), db.GetName())
			if existing, ok := dbs[key]; ok {
				if !proto.Equal(existing, db) {
					return fnerrors.UserError(nil, "%s: database definition for %q is incompatible with %s", alloc.InstanceOwner, db.GetName(), strings.Join(owners[db.GetName()], ","))
				}
			} else {
				dbs[key] = db
				owners[key] = append(owners[key], alloc.InstanceOwner)
			}
			return nil
		}); err != nil {
		return nil, err
	}

	return dbs, nil
}

func (tool) Apply(_ context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	col, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	// TODO: creds should be definable per db instance #217
	var user, password *postgres.Database_Credentials_Secret
	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/db/postgres/opaque/creds") {
		switch secret.Name {
		case "postgres-user-file":
			user = &postgres.Database_Credentials_Secret{
				FromPath: secret.FromPath,
			}
		case "postgres-password-file":
			password = &postgres.Database_Credentials_Secret{
				FromPath: secret.FromPath,
			}
		default:
		}
	}

	inputs, err := collectDatabases(r.Focus.Server, r.PackageOwner())
	if err != nil {
		return err
	}

	dbs := map[string]*postgres.Database{}
	for key, db := range inputs {
		dbs[key] = &postgres.Database{
			Name:     db.Name,
			HostedAt: db.HostedAt,
			Credentials: &postgres.Database_Credentials{
				User:     user,
				Password: password,
			},
		}
	}

	return toolcommon.Apply(r, dbs, postgresType, out)
}

func (tool) Delete(_ context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	return toolcommon.Delete(r, postgresType, out)
}
