// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

type Funcs[M proto.Message] struct {
	Handle    func(context.Context, *schema.SerializedInvocation, M) (*HandleResult, error)
	PlanOrder func(M) (*schema.ScheduleOrder, error)
}

type internalFuncs struct {
	Handle    func(context.Context, *schema.SerializedInvocation, proto.Message) (*HandleResult, error)
	PlanOrder func(proto.Message) (*schema.ScheduleOrder, error)
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
		Handle: func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
			return handle(ctx, def, msg.(M))
		},
		PlanOrder: func(m proto.Message) (*schema.ScheduleOrder, error) {
			return nil, nil
		},
	})
}

func RegisterFuncs[M proto.Message](funcs Funcs[M]) {
	register[M](internalFuncs{
		Handle: func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
			return funcs.Handle(ctx, def, msg.(M))
		},
		PlanOrder: func(msg proto.Message) (*schema.ScheduleOrder, error) {
			if funcs.PlanOrder == nil {
				return nil, nil
			}

			return funcs.PlanOrder(msg.(M))
		},
	})
}

func register[M proto.Message](funcs internalFuncs) {
	tmpl := protos.NewFromType[M]()
	reg := registration{
		key:   protos.TypeUrl(tmpl),
		tmpl:  tmpl,
		funcs: funcs,
	}

	handlers[protos.TypeUrl(tmpl)] = &reg
}

func Compile[M proto.Message](compiler compilerFunc) {
	tmpl := protos.NewFromType[M]()
	compilers[protos.TypeUrl(tmpl)] = compiler
}
