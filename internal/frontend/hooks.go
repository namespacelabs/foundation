// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package frontend

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var registrations struct {
	prepare map[string]PrepareHookFunc
}

type PrepareHook struct {
	Internal string
}

type PrepareProps struct {
	ProvisionInput []*anypb.Any
	Definition     []*schema.Definition
	Extension      []*schema.DefExtension
}

type PrepareHookFunc func(context.Context, ops.Environment, *schema.Server) (*PrepareProps, error)

func RegisterPrepareHook(name string, f PrepareHookFunc) {
	if registrations.prepare == nil {
		registrations.prepare = map[string]PrepareHookFunc{}
	}

	registrations.prepare[name] = f
}

func InvokePrepareHook(ctx context.Context, name string, env ops.Environment, srv *schema.Server) (*PrepareProps, error) {
	if f, ok := registrations.prepare[name]; ok {
		return tasks.Return(ctx, tasks.Action("prepare.invoke-hook").Scope(schema.PackageName(srv.PackageName)).Arg("name", name), func(ctx context.Context) (*PrepareProps, error) {
			return f(ctx, env, srv)
		})
	}

	return nil, fnerrors.UserError(nil, "%s: does not exist", name)
}
