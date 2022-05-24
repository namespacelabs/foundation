// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/philopon/go-toposort"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// Environment represents an execution environment: it puts together a root
// workspace, a workspace configuration (devhost) and then finally the
// schema-level environment we're running for.
type Environment interface {
	fnerrors.Location
	Workspace() *schema.Workspace
	DevHost() *schema.DevHost
	Proto() *schema.Environment // Will be nil if not in a build or deployment phase.
}

// A dispatcher provides the implementation for a particular type, i.e. it
// handles the execution of a particular serialized invocation.
type Dispatcher[M proto.Message] interface {
	Handle(context.Context, Environment, *schema.Definition, M) (*HandleResult, error)
}

// A BatchedDispatcher represents an implementation which batches the execution
// of multiple invocations.
type BatchedDispatcher[M proto.Message] interface {
	StartSession(context.Context, Environment) Session[M]
}

// A session represents a single batched invocation.
type Session[M proto.Message] interface {
	Dispatcher[M]
	Commit() error
}

type HandleResult struct {
	Waiters []Waiter
}

// A plan collects a set of invocations which can then be executed as a batch.
type Plan struct {
	definitions []*schema.Definition
	nodes       []*rnode
	scope       schema.PackageList
}

func NewPlan() *Plan {
	return &Plan{}
}

func (g *Plan) Add(defs ...*schema.Definition) error {
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

func (g *Plan) Execute(ctx context.Context, name string, env Environment) (waiters []Waiter, err error) {
	err = tasks.Action(name).Scope(g.scope.PackageNames()...).Run(ctx,
		func(ctx context.Context) (err error) {
			waiters, err = g.apply(ctx, env, false)
			return
		})
	return
}

func (g *Plan) ExecuteParallel(ctx context.Context, name string, env Environment) (waiters []Waiter, err error) {
	err = tasks.Action(name).Scope(g.scope.PackageNames()...).Run(ctx,
		func(ctx context.Context) (err error) {
			waiters, err = g.apply(ctx, env, true)
			return
		})
	return
}

func (g *Plan) apply(ctx context.Context, env Environment, parallel bool) ([]Waiter, error) {
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

func (g *Plan) Definitions() []*schema.Definition {
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
