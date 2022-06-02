// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"
	"reflect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/schema"
)

type rnode struct {
	def *schema.SerializedInvocation
	reg *registration
	res *HandleResult
	err error // Error captured from a previous run.
}

type registration struct {
	key          string
	tmpl         proto.Message
	dispatcher   dispatcherFunc
	startSession startSessionFunc
	after        []string
}

type dispatcherFunc func(context.Context, Environment, *schema.SerializedInvocation, proto.Message) (*HandleResult, error)
type startSessionFunc func(context.Context, Environment) (dispatcherFunc, commitSessionFunc)
type commitSessionFunc func() error

var handlers = map[string]*registration{}

func Register[M proto.Message](mr Dispatcher[M]) {
	var startSession startSessionFunc
	if stateful, ok := mr.(BatchedDispatcher[M]); ok {
		startSession = func(ctx context.Context, env Environment) (dispatcherFunc, commitSessionFunc) {
			st := stateful.StartSession(ctx, env)
			return func(ctx context.Context, env Environment, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
					return st.Handle(ctx, env, def, msg.(M))
				}, func() error {
					return st.Commit()
				}
		}
	}

	register[M](func(ctx context.Context, env Environment, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
		return mr.Handle(ctx, env, def, msg.(M))
	}, startSession)
}

func RegisterFunc[M proto.Message](mr func(ctx context.Context, env Environment, def *schema.SerializedInvocation, m M) (*HandleResult, error)) {
	register[M](func(ctx context.Context, env Environment, def *schema.SerializedInvocation, msg proto.Message) (*HandleResult, error) {
		return mr(ctx, env, def, msg.(M))
	}, nil)
}

func RunAfter(base, after proto.Message) {
	h := handlers[messageKey(after)]
	h.after = append(h.after, messageKey(base))
}

func register[M proto.Message](dispatcher dispatcherFunc, startSession startSessionFunc) {
	var m M

	tmpl := reflect.New(reflect.TypeOf(m).Elem()).Interface().(proto.Message)
	reg := registration{
		key:          messageKey(tmpl),
		tmpl:         tmpl,
		dispatcher:   dispatcher,
		startSession: startSession,
	}

	handlers[messageKey(tmpl)] = &reg
}

func messageKey(msg proto.Message) string {
	packed, err := anypb.New(msg)
	if err != nil {
		panic(err)
	}
	return packed.GetTypeUrl()
}
