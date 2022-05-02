// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/philopon/go-toposort"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
	"tailscale.com/util/multierr"
)

type Dispatcher[M proto.Message] interface {
	Run(context.Context, Environment, *schema.Definition, M) (*DispatcherResult, error)
}

type Session[M proto.Message] interface {
	Dispatcher[M]
	Commit() error
}

type HasStartSession[M proto.Message] interface {
	StartSession(context.Context, Environment) Session[M]
}

type DispatcherResult struct {
	Waiters []Waiter
}

type Environment interface {
	fnerrors.Location
	Workspace() *schema.Workspace
	DevHost() *schema.DevHost
	Proto() *schema.Environment // Will be nil if not in a build or deployment phase.
}

type Runner struct {
	definitions []*schema.Definition
	nodes       []*rnode
	scope       schema.PackageList
}

type rnode struct {
	def *schema.Definition
	reg *registration
	res *DispatcherResult
	err error // Error captured from a previous run.
}

type registration struct {
	key          string
	tmpl         proto.Message
	dispatcher   dispatcherFunc
	startSession startSessionFunc
	after        []string
}

type dispatcherFunc func(context.Context, Environment, *schema.Definition, proto.Message) (*DispatcherResult, error)
type commitSessionFunc func() error
type startSessionFunc func(context.Context, Environment) (dispatcherFunc, commitSessionFunc)

var handlers = map[string]*registration{}

func Register[M proto.Message](mr Dispatcher[M]) {
	var startSession startSessionFunc
	if stateful, ok := mr.(HasStartSession[M]); ok {
		startSession = func(ctx context.Context, env Environment) (dispatcherFunc, commitSessionFunc) {
			st := stateful.StartSession(ctx, env)
			return func(ctx context.Context, env Environment, def *schema.Definition, msg proto.Message) (*DispatcherResult, error) {
					return st.Run(ctx, env, def, msg.(M))
				}, func() error {
					return st.Commit()
				}
		}
	}

	register[M](func(ctx context.Context, env Environment, def *schema.Definition, msg proto.Message) (*DispatcherResult, error) {
		return mr.Run(ctx, env, def, msg.(M))
	}, startSession)
}

func RegisterFunc[M proto.Message](mr func(ctx context.Context, env Environment, def *schema.Definition, m M) (*DispatcherResult, error)) {
	register[M](func(ctx context.Context, env Environment, def *schema.Definition, msg proto.Message) (*DispatcherResult, error) {
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

func NewRunner() *Runner {
	return &Runner{}
}

func (g *Runner) Add(defs ...*schema.Definition) error {
	var nodes []*rnode
	for _, src := range defs {
		key := src.Impl.GetTypeUrl()
		reg, ok := handlers[key]
		if !ok {
			return fnerrors.InternalError("%v: no handler registered", key)
		}

		nodes = append(nodes, &rnode{
			def: src,
			reg: reg,
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
	err = tasks.Action(name).Scope(g.scope.PackageNames()...).Run(ctx,
		func(ctx context.Context) (err error) {
			waiters, err = g.apply(ctx, env, false)
			return
		})
	return
}

func (g *Runner) ApplyParallel(ctx context.Context, name string, env Environment) (waiters []Waiter, err error) {
	err = tasks.Action(name).Scope(g.scope.PackageNames()...).Run(ctx,
		func(ctx context.Context) (err error) {
			waiters, err = g.apply(ctx, env, true)
			return
		})
	return
}

func (g *Runner) apply(ctx context.Context, env Environment, parallel bool) ([]Waiter, error) {
	err := tasks.Attachments(ctx).AttachSerializable("definitions.json", "fn.graph", g.definitions)
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to serialize graph definition: %v", err)
	}

	sessions := map[string]dispatcherFunc{}
	commits := map[string]commitSessionFunc{}

	nodes, err := topoSortNodes(g.nodes)
	if err != nil {
		return nil, err
	}

	for _, n := range nodes {
		if n.err != nil {
			continue
		}

		typeUrl := n.def.Impl.GetTypeUrl()
		if _, has := sessions[typeUrl]; has {
			continue
		}

		if n.reg.startSession != nil {
			dispatcher, commit := n.reg.startSession(ctx, env)
			sessions[typeUrl] = dispatcher
			commits[typeUrl] = commit
		}
	}

	var errs []error
	var waiters []Waiter
	for _, n := range nodes {
		if n.err != nil {
			continue
		}

		copy := proto.Clone(n.reg.tmpl)
		if err := n.def.Impl.UnmarshalTo(copy); err != nil {
			errs = append(errs, err)
			continue
		}

		dispatcher := n.reg.dispatcher
		typeUrl := n.def.Impl.GetTypeUrl()
		if d, has := sessions[typeUrl]; has {
			dispatcher = d
		}

		d, err := dispatcher(ctx, env, n.def, copy)
		n.res = d
		n.err = err
		if err != nil {
			errs = append(errs, fnerrors.InternalError("failed to run %q: %w", typeUrl, err))
		}
		if d != nil {
			waiters = append(waiters, d.Waiters...)
		}
	}

	// Use insertion order.
	var ordered []commitSessionFunc
	var orderedTypeUrls []string
	for _, n := range nodes {
		typeUrl := n.def.Impl.GetTypeUrl()
		if commit, has := commits[typeUrl]; has {
			ordered = append(ordered, commit)
			orderedTypeUrls = append(orderedTypeUrls, typeUrl)
			delete(sessions, typeUrl)
			delete(commits, typeUrl)
		}
	}

	var ex executor.Executor
	var wait func() error

	if parallel {
		ex, wait = executor.New(ctx)
	} else {
		ex, wait = executor.Serial(ctx)
	}

	for k, commit := range ordered {
		k := k           // Close k.
		commit := commit // Close commit.

		ex.Go(func(ctx context.Context) error {
			if err := commit(); err != nil {
				return fnerrors.InternalError("failed to close %q: %w", orderedTypeUrls[k], err)
			}
			return nil
		})
	}

	if err := wait(); err != nil {
		errs = append(errs, err)
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

func topoSortNodes(nodes []*rnode) ([]*rnode, error) {
	graph := toposort.NewGraph(len(nodes))

	keyTypes := map[string]struct{}{}
	for _, n := range nodes {
		keyTypes[n.reg.key] = struct{}{}
	}

	for k := range keyTypes {
		// key types are always prefixed by an underscore.
		graph.AddNode("_" + k)
	}

	// The idea is that a category is only done when all of its individual nodes are.
	for k, n := range nodes {
		ks := fmt.Sprintf("%d", k)
		graph.AddNode(ks)
		graph.AddEdge(ks, "_"+n.reg.key)

		for _, after := range n.reg.after {
			graph.AddEdge("_"+after, ks)
		}
	}

	result, solved := graph.Toposort()
	if !solved {
		return nil, fnerrors.InternalError("ops dependencies are not solvable")
	}

	end := make([]*rnode, 0, len(nodes))
	for _, k := range result {
		if strings.HasPrefix(k, "_") {
			continue // Was a key.
		}
		i, err := strconv.ParseInt(k, 10, 64)
		if err != nil {
			return nil, fnerrors.InternalError("failed to parse serialized index")
		}
		end = append(end, nodes[i])
	}

	return end, nil
}
