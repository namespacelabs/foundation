// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/allocations"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/toolcommon"
)

const postgresType = "opaque"

type tool struct{}

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	configure.Handle(h)
}

func collectDatabases(server *schema.Server, owner string) (map[string]*postgres.Database, error) {
	dbs := map[string]*postgres.Database{}
	owners := map[string][]string{}
	if err := allocations.Visit(server.Allocation, schema.PackageName(owner), &postgres.Database{},
		func(alloc *schema.Allocation_Instance, instantiate *schema.Instantiate, db *postgres.Database) error {
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

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	initArgs := []string{}

	col, err := secrets.Collect(r.Focus.Server)
	if err != nil {
		return err
	}

	// TODO: creds should be definable per db instance #217
	for _, secret := range col.SecretsOf("namespacelabs.dev/foundation/universe/db/postgres/opaque/creds") {
		switch secret.Name {
		case "postgres-user-file":
			initArgs = append(initArgs, fmt.Sprintf("--postgres_user_file=%s", secret.FromPath))
		case "postgres-password-file":
			initArgs = append(initArgs, fmt.Sprintf("--postgres_password_file=%s", secret.FromPath))
		default:
		}
	}

	dbs, err := collectDatabases(r.Focus.Server, r.PackageOwner())
	if err != nil {
		return err
	}

	return toolcommon.Apply(ctx, r, dbs, postgresType, initArgs, out)
}

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return toolcommon.Delete(r, postgresType, out)
}
