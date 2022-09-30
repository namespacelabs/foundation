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
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/multierr"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type HandleResult struct {
	Waiters []Waiter

	Outputs []Output
}

type Output struct {
	InstanceID string
	Message    proto.Message
}

// A plan collects a set of invocations which can then be executed as a batch.
type Plan struct {
	definitions []*schema.SerializedInvocation
	scope       schema.PackageList
}

type parsedPlan struct {
	definitions []*schema.SerializedInvocation
	nodes       []*rnode
}

type rnode struct {
	def   *schema.SerializedInvocation
	obj   proto.Message
	order *schema.ScheduleOrder
	reg   *registration
	res   *HandleResult
	value any
	err   error // Error captured from a previous run.
}

func NewEmptyPlan() *Plan {
	return &Plan{}
}

func NewPlan(defs ...*schema.SerializedInvocation) *Plan {
	return NewEmptyPlan().Add(defs...)
}

func (g *Plan) Add(defs ...*schema.SerializedInvocation) *Plan {
	g.definitions = append(g.definitions, defs...)
	for _, def := range defs {
		g.scope.AddMultiple(schema.PackageNames(def.Scope...)...)
	}
	return g
}

func compile(ctx context.Context, srcs []*schema.SerializedInvocation) (*parsedPlan, error) {
	g := &parsedPlan{}

	var defs []*schema.SerializedInvocation
	tocompile := map[string][]*schema.SerializedInvocation{}

	for _, src := range srcs {
		if compilers[src.Impl.TypeUrl] != nil {
			tocompile[src.Impl.TypeUrl] = append(tocompile[src.Impl.TypeUrl], src)
		} else {
			defs = append(defs, src)
		}
	}

	compileKeys := maps.Keys(tocompile)
	slices.Sort(compileKeys)

	for _, key := range compileKeys {
		compiled, err := compilers[key](ctx, tocompile[key])
		if err != nil {
			return nil, err
		}
		defs = append(defs, compiled...)
	}

	var nodes []*rnode
	for _, src := range defs {
		key := src.Impl.GetTypeUrl()
		reg, ok := handlers[key]
		if !ok {
			return nil, fnerrors.InternalError("%v: no handler registered", key)
		}

		copy := proto.Clone(reg.tmpl)
		if err := src.Impl.UnmarshalTo(copy); err != nil {
			return nil, fnerrors.InternalError("%v: failed to unmarshal: %w", key, err)
		}

		node := &rnode{
			def: src,
			reg: reg,
			obj: copy,
		}

		if reg.funcs.Parse != nil {
			var err error
			node.value, err = reg.funcs.Parse(ctx, src, copy)
			if err != nil {
				return nil, fnerrors.InternalError("%s: failed to parse: %w", key, err)
			}
		}

		computedOrder, err := reg.funcs.PlanOrder(copy, node.value)
		if err != nil {
			return nil, fnerrors.InternalError("%s: failed to compute order: %w", key, err)
		}

		if computedOrder == nil {
			computedOrder = &schema.ScheduleOrder{}
		}

		computedOrder.SchedCategory = append(computedOrder.SchedCategory, src.Order.GetSchedCategory()...)
		computedOrder.SchedAfterCategory = append(computedOrder.SchedAfterCategory, src.Order.GetSchedAfterCategory()...)

		node.order = computedOrder

		nodes = append(nodes, node)
	}

	g.definitions = append(g.definitions, defs...)
	g.nodes = append(g.nodes, nodes...)
	return g, nil
}

func Serialize(g *Plan) *schema.SerializedProgram {
	return &schema.SerializedProgram{Invocation: g.definitions}
}

type recordedOutput struct {
	Message proto.Message
	Used    bool
}

func (g *parsedPlan) apply(ctx context.Context) ([]Waiter, error) {
	err := tasks.Attachments(ctx).AttachSerializable("definitions.json", "fn.graph", g.definitions)
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to serialize graph definition: %v", err)
	}

	nodes, err := topoSortNodes(ctx, g.nodes)
	if err != nil {
		return nil, err
	}

	var errs []error
	var waiters []Waiter

	outputs := map[string]*recordedOutput{}
	for _, n := range nodes {
		if n.err != nil {
			continue
		}

		typeUrl := n.def.Impl.GetTypeUrl()

		invCtx := ctx
		inputs, err := prepareInputs(outputs, n.def)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		if len(inputs) > 0 {
			invCtx = injectValues(invCtx, InputsInjection.With(inputs))
		}

		n.res, n.err = n.reg.funcs.Handle(invCtx, n.def, n.obj, n.value)
		if n.err != nil {
			errs = append(errs, fnerrors.InternalError("failed to run %q: %w", typeUrl, n.err))
		} else if n.res != nil {
			for _, output := range n.res.Outputs {
				if _, ok := outputs[output.InstanceID]; ok {
					errs = append(errs, fnerrors.InternalError("duplicate result key: %q", output.InstanceID))
				} else {
					outputs[output.InstanceID] = &recordedOutput{
						Message: output.Message,
					}
				}
			}

			waiters = append(waiters, n.res.Waiters...)
		}
	}

	var unusedKeys []string
	for key, output := range outputs {
		if !output.Used {
			unusedKeys = append(unusedKeys, key)
		}
	}

	slices.Sort(unusedKeys)

	if len(unusedKeys) > 0 {
		errs = append(errs, fnerrors.InternalError("unused outputs: %v", unusedKeys))
	}

	return waiters, multierr.New(errs...)
}

func prepareInputs(outputs map[string]*recordedOutput, def *schema.SerializedInvocation) (Inputs, error) {
	var missing []string

	out := map[string]proto.Message{}
	for _, required := range def.RequiredOutput {
		output, ok := outputs[required]
		if !ok {
			missing = append(missing, required)
		} else {
			out[required] = output.Message
			output.Used = true
		}
	}

	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fnerrors.InvocationError("required inputs are missing: %v", missing)
	}

	return out, nil
}

func (g *Plan) Definitions() []*schema.SerializedInvocation {
	return g.definitions
}

func topoSortNodes(ctx context.Context, nodes []*rnode) ([]*rnode, error) {
	graph := toposort.NewGraph(len(nodes))

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

		for _, cat := range n.order.GetSchedCategory() {
			graph.AddEdge(ks, "cat:"+cat)
		}

		for _, cat := range n.order.GetSchedAfterCategory() {
			graph.AddEdge("cat:"+cat, ks)
		}
	}

	sorted, solved := graph.Toposort()
	if !solved {
		fmt.Fprintf(console.Errors(ctx), "execution sort failed:\n")

		for k, n := range nodes {
			fmt.Fprintf(console.Errors(ctx), " #%d %q --> cats:%v after:%v\n", k, n.def.Description,
				n.order.GetSchedCategory(),
				n.order.GetSchedAfterCategory())
		}

		return nil, fnerrors.InternalError("ops dependencies are not solvable")
	}

	var debug bytes.Buffer

	end := make([]*rnode, 0, len(nodes))
	for _, k := range sorted {
		parsed := strings.TrimPrefix(k, "iid:")
		if parsed == k {
			fmt.Fprintf(&debug, " [%s]\n", k)
			continue
		}

		i, err := strconv.ParseInt(parsed, 10, 64)
		if err != nil {
			return nil, fnerrors.InternalError("failed to parse serialized index")
		}
		end = append(end, nodes[i])

		fmt.Fprintf(&debug, " #%d %q --> cats:%v after:%v\n", i, nodes[i].def.Description,
			nodes[i].order.GetSchedCategory(),
			nodes[i].order.GetSchedAfterCategory())
	}

	fmt.Fprintf(console.Debug(ctx), "execution sorted:\n%s", debug.Bytes())

	return end, nil
}
