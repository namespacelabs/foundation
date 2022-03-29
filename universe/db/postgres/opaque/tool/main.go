// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/provision/tool/bootstrap"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/universe/db/postgres"
	"namespacelabs.dev/foundation/universe/db/postgres/toolcommon"
)

type tool struct{}

func main() {
	bootstrap.RunTool(tool{})
}

func collectDatabases(server *schema.Server) (map[schema.PackageName][]*postgres.Database, error) {
	dbs := map[schema.PackageName][]*postgres.Database{}
	for _, alloc := range server.Allocation {
		for _, instance := range alloc.Instance {
			for _, instantiate := range instance.Instantiated {
				if instantiate.GetPackageName() == "namespacelabs.dev/foundation/universe/db/postgres/opaque" && instantiate.GetType() == "Database" {
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

func (tool) Apply(ctx context.Context, r bootstrap.Request, out *bootstrap.ApplyOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return nil
	}

	dbs, err := collectDatabases(r.Focus.Server)
	if err != nil {
		return err
	}

	return toolcommon.Apply(ctx, r, dbs, "opaque", out)
}

func (tool) Delete(ctx context.Context, r bootstrap.Request, out *bootstrap.DeleteOutput) error {
	return toolcommon.Delete(r, "opaque", out)
}