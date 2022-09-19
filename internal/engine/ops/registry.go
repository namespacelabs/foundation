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

type rnode struct {
	def   *schema.SerializedInvocation
	obj   proto.Message
	order *schema.ScheduleOrder
	reg   *registration
	res   *HandleResult
	err   error // Error captured from a previous run.
}

type registration struct {
	key          string
	tmpl         proto.Message
	dispatcher   dispatcherFunc
	planOrder    planOrderFunc
	startSession startSessionFunc
}

type dispatcherFunc func(context.Context, *schema.SerializedInvocation, proto.Message) (*HandleResult, error)
type planOrderFunc func(proto.Message) (*schema.ScheduleOrder, error)
type startSessionFunc func(context.Context) (dispatcherFunc, commitSessionFunc, error)
type commitSessionFunc func() error
type compilerFunc func(context.Context, []*schema.SerializedInvocation) ([]*schema.SerializedInvocation, error)

var (
	handlers  = map[string]*registration{}
	compilers = map[string]compilerFunc{}
)

func Register[M proto.Message](mr Dispatcher[M]) {
	var startSession startSessionFunc
	if stateful, ok := mr.(BatchedDispatcher[M]); ok {
		startSession = func(ctx context.Context) (dispatcherFunc, commitSessionFunc, error) {
			st, err := stateful.StartSession(ctx)
			if err != nil {
				return nil, nil, err
			}
			return func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
					return st.Handle(ctx, def, msg.(M))
				}, func() error {
					return st.Commit()
				}, nil
		}
	}

	register[M](func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
		return mr.Handle(ctx, def, msg.(M))
	}, func(m proto.Message) (*schema.ScheduleOrder, error) {
		return mr.PlanOrder(m.(M))
	}, startSession)
}

func RegisterHandlerFunc[M proto.Message](handle func(context.Context, *schema.SerializedInvocation, M) (*HandleResult, error)) {
	register[M](func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
		return handle(ctx, def, msg.(M))
	}, func(m proto.Message) (*schema.ScheduleOrder, error) {
		return nil, nil
	}, nil)
}

func Compile[M proto.Message](compiler compilerFunc) {
	tmpl := protos.NewFromType[M]()
	compilers[protos.TypeUrl(tmpl)] = compiler
}

type Funcs[M proto.Message] struct {
	Handle    func(context.Context, *schema.SerializedInvocation, M) (*HandleResult, error)
	PlanOrder func(M) (*schema.ScheduleOrder, error)
}

func RegisterFuncs[M proto.Message](funcs Funcs[M]) {
	register[M](func(ctx context.Context, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
		return funcs.Handle(ctx, def, msg.(M))
	}, func(m proto.Message) (*schema.ScheduleOrder, error) {
		if funcs.PlanOrder == nil {
			return nil, nil
		}

		return funcs.PlanOrder(m.(M))
	}, nil)
}

func register[M proto.Message](dispatcher dispatcherFunc, planOrder planOrderFunc, startSession startSessionFunc) {
	tmpl := protos.NewFromType[M]()
	reg := registration{
		key:          protos.TypeUrl(tmpl),
		tmpl:         tmpl,
		dispatcher:   dispatcher,
		planOrder:    planOrder,
		startSession: startSession,
	}

	handlers[protos.TypeUrl(tmpl)] = &reg
}
