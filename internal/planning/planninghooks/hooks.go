// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planninghooks

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

var registrations struct {
	prepare map[string]PrepareHookFunc
}

type ProvisionResult struct {
	ProvisionInput           []rtypes.ProvisionInput
	SerializedProvisionInput []*schema.SerializedMessage
	Extension                []*schema.DefExtension
	ServerExtension          []*schema.ServerExtension
}

type InternalPrepareProps struct {
	pkggraph.PreparedProvisionPlan
	ProvisionResult
}

func (p *InternalPrepareProps) AppendWith(rhs InternalPrepareProps) {
	p.PreparedProvisionPlan.AppendWith(rhs.PreparedProvisionPlan)
	p.ProvisionResult.AppendWith(rhs.ProvisionResult)
}

func (p *ProvisionResult) AppendWith(rhs ProvisionResult) {
	p.ProvisionInput = append(p.ProvisionInput, rhs.ProvisionInput...)
	p.SerializedProvisionInput = append(p.SerializedProvisionInput, rhs.SerializedProvisionInput...)
	p.Extension = append(p.Extension, rhs.Extension...)
	p.ServerExtension = append(p.ServerExtension, rhs.ServerExtension...)
}

type PrepareHookFunc func(context.Context, cfg.Context, *schema.Stack_Entry) (*InternalPrepareProps, error)

func RegisterPrepareHook(name string, f PrepareHookFunc) {
	if registrations.prepare == nil {
		registrations.prepare = map[string]PrepareHookFunc{}
	}

	registrations.prepare[name] = f
}

func InvokeInternalPrepareHook(ctx context.Context, name string, env cfg.Context, srv *schema.Stack_Entry) (*InternalPrepareProps, error) {
	if f, ok := registrations.prepare[name]; ok {
		return tasks.Return(ctx, tasks.Action("prepare.invoke-hook").Scope(srv.GetPackageName()).Arg("name", name), func(ctx context.Context) (*InternalPrepareProps, error) {
			return f(ctx, env, srv)
		})
	}

	return nil, fnerrors.New(fmt.Sprintf("%s: does not exist", name))
}
