// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package execution

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
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/tasks"
)

type HandleResult struct {
	Waiter  Waiter
	Outputs []Output
}

type Output struct {
	InstanceID string
	Message    proto.Message
	Instance   any
}

// A plan collects a set of invocations which can then be executed as a batch.
type Plan struct {
	definitions []*schema.SerializedInvocation
	scope       schema.PackageList
}

type compiledPlan struct {
	definitions []*schema.SerializedInvocation
	nodes       []*executionNode
}

type executionNode struct {
	invocation    *schema.SerializedInvocation
	message       proto.Message
	parsed        any
	computedOrder *schema.ScheduleOrder
	dispatch      internalFuncs
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

type compileOpts struct {
	OrchestratorVersion int32
}

func compile(ctx context.Context, srcs []*schema.SerializedInvocation, opts compileOpts) (*compiledPlan, error) {
	g := &compiledPlan{}

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

	var nodes []*executionNode
	for _, src := range defs {
		key := src.Impl.GetTypeUrl()
		humanKey := strings.TrimPrefix(key, "type.googleapis.com/")

		if src.MinimumVersion > opts.OrchestratorVersion {
			return nil, fnerrors.InternalError("%v: requires orchestrator version %d (got %d)", humanKey, src.MinimumVersion, opts.OrchestratorVersion)
		}

		reg, ok := handlers[key]
		if !ok {
			return nil, fnerrors.InternalError("%v: no handler registered", humanKey)
		}

		msg, err := reg.unmarshal(src)
		if err != nil {
			return nil, fnerrors.InternalError("%v: failed to unmarshal: %w", humanKey, err)
		}

		node := &executionNode{
			invocation: src,
			dispatch:   reg.funcs,
			message:    msg,
		}

		if reg.funcs.Parse != nil {
			var err error
			node.parsed, err = reg.funcs.Parse(ctx, src, msg)
			if err != nil {
				return nil, fnerrors.InternalError("%s: failed to parse: %w", humanKey, err)
			}
		}

		computedOrder, err := reg.funcs.PlanOrder(ctx, msg, node.parsed)
		if err != nil {
			return nil, fnerrors.InternalError("%s: failed to compute order: %w", humanKey, err)
		}

		if computedOrder == nil {
			computedOrder = &schema.ScheduleOrder{}
		}

		computedOrder.SchedCategory = append(computedOrder.SchedCategory, src.Order.GetSchedCategory()...)
		computedOrder.SchedAfterCategory = append(computedOrder.SchedAfterCategory, src.Order.GetSchedAfterCategory()...)

		node.computedOrder = computedOrder

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
	Message  proto.Message
	Instance any
	Used     bool
}

func (g *compiledPlan) apply(ctx context.Context, ch chan *orchestration.Event, opts ExecuteOpts) error {
	err := tasks.Attachments(ctx).AttachSerializable("definitions.json", "fn.graph", g.definitions)
	if err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to serialize graph definition: %v", err)
	}

	nodes, err := topoSortNodes(ctx, g.nodes)
	if err != nil {
		return err
	}

	var errs []error

	if ch != nil {
		for _, node := range nodes {
			if node.dispatch.EmitStart != nil {
				node.dispatch.EmitStart(ctx, node.invocation, node.message, node.parsed, ch)
			}
		}
	}

	ctx, done := context.WithCancel(ctx)
	defer done()

	outputs := map[string]*recordedOutput{}
	for _, n := range nodes {
		typeUrl := n.invocation.Impl.GetTypeUrl()

		fmt.Fprintf(console.Debug(ctx), "executing %q (%s)\n", typeUrl, n.invocation.Description)

		inputs, err := prepareInputs(outputs, n.invocation)
		if err != nil {
			if opts.ContinueOnErrors {
				errs = append(errs, err)
			} else {
				return err
			}
			continue
		}

		invCtx := injectValues(ctx, InputsInjection.With(inputs))

		var running *tasks.RunningAction
		if opts.WrapWithActions || opts.TaskMake != nil {
			var action *tasks.ActionEvent
			if opts.TaskMake != nil {
				action = opts.TaskMake(n.invocation)
			} else {
				action = tasks.Action("execute.step").Arg("typeUrl", typeUrl)
			}

			if n.invocation.Description != "" {
				action = action.HumanReadable(n.invocation.Description)
			}

			if opts.TaskOnDone != nil {
				action = action.OnDone(opts.TaskOnDone)
			}

			invCtx, running = action.Start(invCtx, opts.TaskTracer)
		}

		res, err := n.dispatch.Handle(invCtx, n.invocation, n.message, n.parsed, ch)

		if running != nil {
			_ = running.Done(err)
		}

		if err != nil {
			wrappedErr := fnerrors.InternalError("failed to run %q: %w", typeUrl, err)
			if opts.ContinueOnErrors {
				errs = append(errs, wrappedErr)
			} else {
				return wrappedErr
			}
		} else if res != nil {
			for _, output := range res.Outputs {
				if _, ok := outputs[output.InstanceID]; ok {
					wrappedErr := fnerrors.InternalError("duplicate result key: %q", output.InstanceID)
					if opts.ContinueOnErrors {
						errs = append(errs, wrappedErr)
					} else {
						return wrappedErr
					}
				} else {
					outputs[output.InstanceID] = &recordedOutput{
						Message:  output.Message,
						Instance: output.Instance,
					}
				}
			}

			if res.Waiter != nil {
				if opts.OnWaiter != nil {
					opts.OnWaiter(ctx, res.Waiter)
				} else {
					fmt.Fprintf(console.Debug(ctx), "%s: ignoring waiter\n", typeUrl)
				}
			}
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

	return multierr.New(errs...)
}

func prepareInputs(outputs map[string]*recordedOutput, def *schema.SerializedInvocation) (Inputs, error) {
	var missing []string

	out := Inputs{}
	for _, required := range def.RequiredOutput {
		output, ok := outputs[required]
		if !ok {
			missing = append(missing, required)
		} else {
			out[required] = Input{
				Message:  output.Message,
				Instance: output.Instance,
			}
			output.Used = true
		}
	}

	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fnerrors.InternalError("required inputs are missing: %v", missing)
	}

	return out, nil
}

func (g *Plan) Definitions() []*schema.SerializedInvocation {
	return g.definitions
}

func topoSortNodes(ctx context.Context, nodes []*executionNode) ([]*executionNode, error) {
	graph := toposort.NewGraph(len(nodes))

	categories := map[string]struct{}{}
	for _, n := range nodes {
		for _, cat := range n.computedOrder.GetSchedCategory() {
			if _, has := categories[cat]; !has {
				graph.AddNode("cat:" + cat)
				categories[cat] = struct{}{}
			}
		}

		for _, cat := range n.computedOrder.GetSchedAfterCategory() {
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

		for _, cat := range n.computedOrder.GetSchedCategory() {
			graph.AddEdge(ks, "cat:"+cat)
		}

		for _, cat := range n.computedOrder.GetSchedAfterCategory() {
			graph.AddEdge("cat:"+cat, ks)
		}
	}

	sorted, solved := graph.Toposort()
	if !solved {
		for k, n := range nodes {
			fmt.Fprintf(console.Errors(ctx), " #%d %q --> cats:%v after:%v\n", k, n.invocation.Description,
				n.computedOrder.GetSchedCategory(),
				n.computedOrder.GetSchedAfterCategory())
		}

		return nil, fnerrors.InternalError("ops dependencies are not solvable")
	}

	var debug bytes.Buffer

	end := make([]*executionNode, 0, len(nodes))
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

		fmt.Fprintf(&debug, " #%d %q --> cats:%v after:%v\n", i, nodes[i].invocation.Description,
			nodes[i].computedOrder.GetSchedCategory(),
			nodes[i].computedOrder.GetSchedAfterCategory())
	}

	fmt.Fprintf(console.Debug(ctx), "execution sorted:\n%s", debug.Bytes())

	return end, nil
}
