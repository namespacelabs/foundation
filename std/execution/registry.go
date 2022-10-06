// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package execution

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
)

type Funcs[M proto.Message] struct {
	Handle    func(context.Context, *schema.SerializedInvocation, M) (*HandleResult, error)
	PlanOrder func(M) (*schema.ScheduleOrder, error)
}

type VFuncs[M proto.Message, V any] struct {
	Parse     func(context.Context, *schema.SerializedInvocation, M) (V, error)
	Handle    func(context.Context, *schema.SerializedInvocation, V) (*HandleResult, error)
	PlanOrder func(V) (*schema.ScheduleOrder, error)
}

type internalFuncs struct {
	Parse     func(context.Context, *schema.SerializedInvocation, proto.Message) (any, error)
	Handle    func(context.Context, *schema.SerializedInvocation, proto.Message, any) (*HandleResult, error)
	PlanOrder func(proto.Message, any) (*schema.ScheduleOrder, error)
}

type compilerFunc func(context.Context, []*schema.SerializedInvocation) ([]*schema.SerializedInvocation, error)

type registration struct {
	key   string
	tmpl  proto.Message
	funcs internalFuncs
}

var (
	handlers  = map[string]*registration{}
	compilers = map[string]compilerFunc{}
)

func RegisterHandlerFunc[M proto.Message](handle func(context.Context, *schema.SerializedInvocation, M) (*HandleResult, error)) {
	register[M](internalFuncs{
		Handle: func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message, _ any) (*HandleResult, error) {
			return handle(ctx, def, msg.(M))
		},
		PlanOrder: func(proto.Message, any) (*schema.ScheduleOrder, error) {
			return nil, nil
		},
	})
}

func RegisterFuncs[M proto.Message](funcs Funcs[M]) {
	register[M](internalFuncs{
		Handle: func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message, _ any) (*HandleResult, error) {
			return funcs.Handle(ctx, def, msg.(M))
		},
		PlanOrder: func(msg proto.Message, _ any) (*schema.ScheduleOrder, error) {
			if funcs.PlanOrder == nil {
				return nil, nil
			}

			return funcs.PlanOrder(msg.(M))
		},
	})
}

func RegisterVFuncs[M proto.Message, V any](funcs VFuncs[M, V]) {
	register[M](internalFuncs{
		Parse: func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message) (any, error) {
			return funcs.Parse(ctx, def, msg.(M))
		},
		Handle: func(ctx context.Context, def *schema.SerializedInvocation, _ proto.Message, value any) (*HandleResult, error) {
			return funcs.Handle(ctx, def, value.(V))
		},
		PlanOrder: func(_ proto.Message, value any) (*schema.ScheduleOrder, error) {
			if funcs.PlanOrder == nil {
				return nil, nil
			}

			return funcs.PlanOrder(value.(V))
		},
	})
}

func register[M proto.Message](funcs internalFuncs) {
	reg := registration{
		key:   protos.TypeUrl[M](),
		tmpl:  protos.NewFromType[M](),
		funcs: funcs,
	}

	handlers[protos.TypeUrl[M]()] = &reg
}

func Compile[M proto.Message](compiler compilerFunc) {
	compilers[protos.TypeUrl[M]()] = compiler
}
