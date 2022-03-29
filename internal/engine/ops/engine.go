// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"
	"io"

	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/tasks"
	"tailscale.com/util/multierr"
)

type Dispatcher interface {
	Run(context.Context, Environment, *schema.Definition, proto.Message) (*DispatcherResult, error)
}

type DispatcherCloser interface {
	Dispatcher
	io.Closer
}

type IsStateful interface {
	StartSession(context.Context, Environment) DispatcherCloser
}

type DispatcherResult struct {
	Waiters []Waiter
}

type DispatcherFunc func(context.Context, Environment, *schema.Definition, proto.Message) (*DispatcherResult, error)

type Environment interface {
	fnerrors.Location
	Workspace() *schema.Workspace
	DevHost() *schema.DevHost
	Proto() *schema.Environment // Will be nil if not in a build or deployment phase.
}

type WorkspaceEnvironment interface {
	workspace.Packages
	OutputFS() fnfs.ReadWriteFS
}

func (f DispatcherFunc) Run(ctx context.Context, env Environment, def *schema.Definition, m proto.Message) (*DispatcherResult, error) {
	return f(ctx, env, def, m)
}

type Runner struct {
	definitions []*schema.Definition
	nodes       []rnode
	scope       schema.PackageList
}

type rnode struct {
	def        *schema.Definition
	tmpl       proto.Message
	dispatcher Dispatcher
	res        *DispatcherResult
	err        error
}

type dispatcherRegistration struct {
	tmpl       proto.Message
	dispatcher Dispatcher
}

var handlers = map[string]dispatcherRegistration{}

func Register(msg proto.Message, mr Dispatcher) {
	p, err := anypb.New(msg)
	if err != nil {
		panic(err)
	}

	handlers[p.GetTypeUrl()] = dispatcherRegistration{
		tmpl:       msg,
		dispatcher: mr,
	}
}

func NewRunner() *Runner {
	return &Runner{}
}

func (g *Runner) Add(defs ...*schema.Definition) error {
	var nodes []rnode
	for _, src := range defs {
		reg, ok := handlers[src.Impl.GetTypeUrl()]
		if !ok {
			return fnerrors.InternalError("%v: no handler registered", src.Impl.GetTypeUrl())
		}

		nodes = append(nodes, rnode{
			def:        src,
			tmpl:       reg.tmpl,
			dispatcher: reg.dispatcher,
		})

		for _, scope := range src.Scope {
			g.scope.Add(schema.PackageName(scope))
		}
	}
	g.definitions = append(g.definitions, defs...)
	g.nodes = append(g.nodes, nodes...)
	return nil
}

func (g *Runner) Apply(ctx context.Context, name string, env Environment) (waiters []Waiter, err error) {
	err = tasks.Task(name).Scope(g.scope.PackageNames()...).Run(ctx,
		func(ctx context.Context) (err error) {
			waiters, err = g.apply(ctx, env)
			return
		})
	return
}

func (g *Runner) apply(ctx context.Context, env Environment) ([]Waiter, error) {
	tasks.Attachments(ctx).AttachSerializable("definitions.json", "fn.graph", g.definitions)

	sessions := map[string]DispatcherCloser{}

	for _, n := range g.nodes {
		if n.err != nil {
			continue
		}

		typeUrl := n.def.Impl.GetTypeUrl()
		if _, has := sessions[typeUrl]; has {
			continue
		}

		if stateful, ok := n.dispatcher.(IsStateful); ok {
			sessions[typeUrl] = stateful.StartSession(ctx, env)
		}
	}

	var errs []error
	var waiters []Waiter
	for k, n := range g.nodes {
		if n.err != nil {
			continue
		}

		copy := proto.Clone(n.tmpl)
		if err := n.def.Impl.UnmarshalTo(copy); err != nil {
			errs = append(errs, err)
			continue
		}

		dispatcher := n.dispatcher
		typeUrl := n.def.Impl.GetTypeUrl()
		if d, has := sessions[typeUrl]; has {
			dispatcher = d
		}

		d, err := dispatcher.Run(ctx, env, n.def, copy)
		g.nodes[k].res = d
		g.nodes[k].err = err
		if err != nil {
			errs = append(errs, fnerrors.InternalError("failed to run %q: %w", typeUrl, err))
		}
		if d != nil {
			waiters = append(waiters, d.Waiters...)
		}
	}

	// Use insertion order.
	for _, n := range g.nodes {
		typeUrl := n.def.Impl.GetTypeUrl()
		if closer, has := sessions[typeUrl]; has {
			if err := closer.Close(); err != nil {
				errs = append(errs, fnerrors.InternalError("failed to close %q: %w", typeUrl, err))
			}
			delete(sessions, typeUrl)
		}
	}

	return waiters, multierr.New(errs...)
}

func (g *Runner) Definitions() []*schema.Definition {
	var defs []*schema.Definition
	for _, n := range g.nodes {
		defs = append(defs, n.def)
	}
	return defs
}