// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package ops

import (
	"bytes"
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

type Handler[M proto.Message] interface {
	Handle(context.Context, *schema.SerializedInvocation, M) (*HandleResult, error)
}

// A dispatcher provides the implementation for a particular type, i.e. it
// handles the execution of a particular serialized invocation.
type Dispatcher[M proto.Message] interface {
	Handler[M]

	PlanOrder(M) (*schema.ScheduleOrder, error)
}

// A BatchedDispatcher represents an implementation which batches the execution
// of multiple invocations.
type BatchedDispatcher[M proto.Message] interface {
	StartSession(context.Context) (Session[M], error)
}

// A session represents a single batched invocation.
type Session[M proto.Message] interface {
	Handler[M]
	Commit() error
}

type HandleResult struct {
	Waiters []Waiter
}

// A plan collects a set of invocations which can then be executed as a batch.
type Plan struct {
	definitions []*schema.SerializedInvocation
	nodes       []*rnode
	scope       schema.PackageList
	parallel    bool // Execute invocations in parallel regardless of dependency graph.
}

func NewEmptyPlan() *Plan {
	return &Plan{}
}

func NewPlan(defs ...*schema.SerializedInvocation) (*Plan, error) {
	p := NewEmptyPlan()
	return p, p.Add(defs...)
}

// Don't use this if you don't know why you need it. Use NewPlan instead.
func NewParallelPlan(defs ...*schema.SerializedInvocation) (*Plan, error) {
	p := NewEmptyPlan()
	p.parallel = true
	return p, p.Add(defs...)
}

func (g *Plan) Add(defs ...*schema.SerializedInvocation) error {
	var nodes []*rnode
	for _, src := range defs {
		key := src.Impl.GetTypeUrl()
		reg, ok := handlers[key]
		if !ok {
			return fnerrors.InternalError("%v: no handler registered", key)
		}

		copy := proto.Clone(reg.tmpl)
		if err := src.Impl.UnmarshalTo(copy); err != nil {
			return fnerrors.InternalError("%v: failed to unmarshal: %w", key, err)
		}

		node := &rnode{
			def: src,
			reg: reg,
			obj: copy,
		}

		if src.Order != nil {
			node.order = src.Order
		} else {
			var err error
			node.order, err = reg.planOrder(copy)
			if err != nil {
				return fnerrors.InternalError("%s: failed to compute order: %w", key, err)
			}
		}

		nodes = append(nodes, node)

		for _, scope := range src.Scope {
			g.scope.Add(schema.PackageName(scope))
		}
	}
	g.definitions = append(g.definitions, defs...)
	g.nodes = append(g.nodes, nodes...)
	return nil
}

func Serialize(g *Plan) *schema.SerializedProgram {
	return &schema.SerializedProgram{Invocation: g.definitions}
}

func (g *Plan) apply(ctx context.Context) ([]Waiter, error) {
	err := tasks.Attachments(ctx).AttachSerializable("definitions.json", "fn.graph", g.definitions)
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to serialize graph definition: %v", err)
	}

	sessions := map[string]dispatcherFunc{}
	commits := map[string]commitSessionFunc{}

	nodes, err := topoSortNodes(ctx, g.nodes)
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
			dispatcher, commit, err := n.reg.startSession(ctx)
			if err != nil {
				return nil, err
			}
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

		dispatcher := n.reg.dispatcher
		typeUrl := n.def.Impl.GetTypeUrl()
		if d, has := sessions[typeUrl]; has {
			dispatcher = d
		}

		d, err := dispatcher(ctx, n.def, n.obj)
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

	var ex executor.ExecutorLike

	if g.parallel {
		ex = executor.New(ctx, "plan.apply")
	} else {
		ex = executor.NewSerial(ctx)
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

	if err := ex.Wait(); err != nil {
		errs = append(errs, err)
	}

	return waiters, multierr.New(errs...)
}

func (g *Plan) Definitions() []*schema.SerializedInvocation {
	var defs []*schema.SerializedInvocation
	for _, n := range g.nodes {
		defs = append(defs, n.def)
	}
	return defs
}

func topoSortNodes(ctx context.Context, nodes []*rnode) ([]*rnode, error) {
	graph := toposort.NewGraph(len(nodes))

	keyTypes := map[string]struct{}{}
	for _, n := range nodes {
		keyTypes[n.reg.key] = struct{}{}
	}

	for k := range keyTypes {
		graph.AddNode("key:" + k)
	}

	categories := map[string]struct{}{}
	for _, n := range nodes {
		for _, cat := range n.order.GetSchedCategory() {
			if _, has := categories[cat]; !has {
				graph.AddNode("cat:" + cat)
				categories[cat] = struct{}{}
			}
		}

		for _, cat := range n.order.GetSchedAfterCategory() {
			if _, has := categories[cat]; !has {
				graph.AddNode("cat:" + cat)
				categories[cat] = struct{}{}
			}
		}
	}

	// The idea is that a category is only done when all of its individual nodes are.
	for k, n := range nodes {
		ks := fmt.Sprintf("iid:%d", k)

		graph.AddNode(ks)
		graph.AddEdge(ks, "key:"+n.reg.key)

		for _, cat := range n.order.GetSchedCategory() {
			graph.AddEdge(ks, "cat:"+cat)
		}

		for _, cat := range n.order.GetSchedAfterCategory() {
			graph.AddEdge("cat:"+cat, ks)
		}
	}

	sorted, solved := graph.Toposort()
	if !solved {
		return nil, fnerrors.InternalError("ops dependencies are not solvable")
	}

	var debug bytes.Buffer

	end := make([]*rnode, 0, len(nodes))
	for _, k := range sorted {
		parsed := strings.TrimPrefix(k, "iid:")
		if parsed == k {
			fmt.Fprintf(&debug, " %s", k)
			continue
		}

		i, err := strconv.ParseInt(parsed, 10, 64)
		if err != nil {
			return nil, fnerrors.InternalError("failed to parse serialized index")
		}
		end = append(end, nodes[i])

		fmt.Fprintf(&debug, " %s (%s)", k, strBit(nodes[i].def.Description, 32))
	}

	fmt.Fprintf(console.Debug(ctx), "execution sorted:%s\n", debug.Bytes())

	return end, nil
}

func strBit(str string, n int) string {
	if len(str) > n {
		return str[:n]
	}
	return str
}
