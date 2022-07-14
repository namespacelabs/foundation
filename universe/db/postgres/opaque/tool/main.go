// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/internal/toolcommon"
)

type tool struct{}

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	configure.Handle(h)
}

func collectDatabases(server *schema.Server, owner string) (map[schema.PackageName][]*postgres.Database, error) {
	dbs := map[schema.PackageName][]*postgres.Database{}
	for _, alloc := range server.Allocation {
		for _, instance := range alloc.Instance {
			for _, instantiate := range instance.Instantiated {
				if instantiate.GetPackageName() == owner && instantiate.GetType() == "Database" {
					db := postgres.Database{}
					if err := proto.Unmarshal(instantiate.Constructor.Value, &db); err != nil {
						return nil, err
					}
					dbs[schema.PackageName(instance.InstanceOwner)] = append(dbs[schema.PackageName(instance.InstanceOwner)], &db)
				}
			}
		}
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

	return toolcommon.Apply(ctx, r, dbs, "opaque", initArgs, out)
}

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return toolcommon.Delete(r, "opaque", out)
}
