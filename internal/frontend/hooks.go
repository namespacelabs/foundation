// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package frontend

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var registrations struct {
	prepare map[string]PrepareHookFunc
}

type PrepareHook struct {
	InvokeInternal string
	InvokeBinary   *schema.Invocation
}

type PrepareProps struct {
	PreparedProvisionPlan
	ProvisionInput  []*anypb.Any
	Invocations     []*schema.SerializedInvocation
	Extension       []*schema.DefExtension
	ServerExtension []*schema.ServerExtension
}

func (p *PrepareProps) AppendWith(rhs PrepareProps) {
	p.PreparedProvisionPlan.AppendWith(rhs.PreparedProvisionPlan)
	p.ProvisionInput = append(p.ProvisionInput, rhs.ProvisionInput...)
	p.Invocations = append(p.Invocations, rhs.Invocations...)
	p.Extension = append(p.Extension, rhs.Extension...)
	p.ServerExtension = append(p.ServerExtension, rhs.ServerExtension...)
}

func (p *PrepareProps) AppendInputs(msgs ...proto.Message) error {
	for _, m := range msgs {
		any, err := anypb.New(m)
		if err != nil {
			return err
		}
		p.ProvisionInput = append(p.ProvisionInput, any)
	}
	return nil
}

type PrepareHookFunc func(context.Context, planning.Context, *schema.Stack_Entry) (*PrepareProps, error)

func RegisterPrepareHook(name string, f PrepareHookFunc) {
	if registrations.prepare == nil {
		registrations.prepare = map[string]PrepareHookFunc{}
	}

	registrations.prepare[name] = f
}

func InvokeInternalPrepareHook(ctx context.Context, name string, env planning.Context, srv *schema.Stack_Entry) (*PrepareProps, error) {
	if f, ok := registrations.prepare[name]; ok {
		return tasks.Return(ctx, tasks.Action("prepare.invoke-hook").Scope(srv.GetPackageName()).Arg("name", name), func(ctx context.Context) (*PrepareProps, error) {
			return f(ctx, env, srv)
		})
	}

	return nil, fnerrors.New(fmt.Sprintf("%s: does not exist", name))
}
