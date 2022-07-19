// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"strings"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/allocations"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
	pgconfigure "namespacelabs.dev/foundation/universe/db/postgres/incluster/configure"
)

type tool struct{}

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	configure.Handle(h)
}

func (tool) Apply(_ context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
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

	return pgconfigure.Apply(r, dbs, owners, out)
}

func (tool) Delete(_ context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return pgconfigure.Delete(r, out)
}
