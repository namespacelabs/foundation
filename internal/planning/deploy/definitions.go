// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"fmt"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/philopon/go-toposort"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/tool"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/types"
)

func invokeHandlers(ctx context.Context, env cfg.Context, planner runtime.Planner, stack *planning.Stack, handlers []*tool.Definition, event protocol.Lifecycle) (compute.Computable[*handlerResult], error) {
	props, err := planner.PrepareProvision(ctx)
	if err != nil {
		return nil, err
	}

	propsPerServer := map[schema.PackageName]tool.InvokeProps{}

	definitions := props.Invocation
	extensions := props.Extension
	serverExtensions := props.ServerExtension

	for k, srv := range stack.Servers {
		invokeProps := tool.InvokeProps{Event: event}
		anys, err := expandAnys(props.ProvisionInput)
		if err != nil {
			return nil, err
		}
		invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, anys...)

		for _, dep := range srv.ParsedDeps {
			// XXX breaks isolation.
			for _, input := range dep.PrepareProps.SerializedProvisionInput {
				for _, name := range input.Name {
					invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, &anypb.Any{
						TypeUrl: protos.TypeUrlPrefix + name,
						Value:   input.Value,
					})
				}
			}

			anys, err := expandAnys(dep.PrepareProps.ProvisionInput)
			if err != nil {
				return nil, err
			}
			invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, anys...)

			extensions = append(extensions, dep.PrepareProps.Extension...)
			serverExtensions = append(serverExtensions, dep.PrepareProps.ServerExtension...)
		}

		propsPerServer[stack.Servers[k].PackageName()] = invokeProps
	}

	var invocations []compute.Computable[*protocol.ToolResponse]
	for _, r := range handlers {
		focus, ok := stack.Get(r.TargetServer)
		if !ok {
			return nil, fnerrors.InternalError("found lifecycle for %q, but no such server in our stack", r.TargetServer)
		}

		inv, err := tool.MakeInvocation(ctx, env, planner, r, stack.Proto(), focus.PackageName(), propsPerServer[focus.PackageName()])
		if err != nil {
			return nil, err
		}
		invocations = append(invocations, inv)
	}

	return &finishInvokeHandlers{
		stack:            stack,
		handlers:         handlers,
		invocations:      invocations,
		event:            event,
		definitions:      definitions,
		extensions:       extensions,
		serverExtensions: serverExtensions,
	}, nil
}

func expandAnys(inputs []rtypes.ProvisionInput) ([]*anypb.Any, error) {
	var anys []*anypb.Any
	for _, input := range inputs {
		serialized, err := anypb.New(input.Message)
		if err != nil {
			return nil, fnerrors.InternalError("failed to serialize input: %w", err)
		}

		anys = append(anys, serialized)

		for _, name := range input.Aliases {
			anys = append(anys, &anypb.Any{
				TypeUrl: protos.TypeUrlPrefix + name,
				Value:   serialized.Value,
			})
		}
	}
	return anys, nil
}

type handlerResult struct {
	Stack *planning.Stack
	// Merged set of invocations that are produced from the handlers invoked.
	// Topologically ordered based on the server dependency graph.
	OrderedInvocations []*schema.SerializedInvocation
	ProvisionOutput    map[schema.PackageName]*provisionOutput // Per server.
}

func (hr handlerResult) MergedComputedConfigurations() *schema.ComputedConfigurations {
	computed := &schema.ComputedConfigurations{}
	for _, srv := range hr.ProvisionOutput {
		computed.Entry = append(computed.Entry, srv.ComputedConfigurations.GetEntry()...)
	}

	slices.SortFunc(computed.Entry, func(a, b *schema.ComputedConfigurations_Entry) bool {
		return strings.Compare(a.ServerPackage, b.ServerPackage) < 0
	})

	return computed
}

type provisionOutput struct {
	ComputedConfigurations *schema.ComputedConfigurations
	ServerExtensions       []*schema.ServerExtension
	Extensions             []*schema.DefExtension
}

type finishInvokeHandlers struct {
	stack            *planning.Stack
	handlers         []*tool.Definition
	invocations      []compute.Computable[*protocol.ToolResponse]
	event            protocol.Lifecycle
	definitions      []*schema.SerializedInvocation
	extensions       []*schema.DefExtension
	serverExtensions []*schema.ServerExtension

	compute.LocalScoped[*handlerResult]
}

func (r *finishInvokeHandlers) Action() *tasks.ActionEvent {
	return tasks.Action("provision.invoke-handlers")
}

func (r *finishInvokeHandlers) Inputs() *compute.In {
	in := compute.Inputs().
		Indigestible("stack", r.stack).
		Indigestible("handlers", r.handlers).
		JSON("event", r.event).
		JSON("definitions", r.definitions).
		JSON("extensions", r.extensions).
		JSON("serverExtensions", r.serverExtensions)
	for k, invocation := range r.invocations {
		in = in.Computable(fmt.Sprintf("invocation%d", k), invocation)
	}
	return in
}

func (r *finishInvokeHandlers) Output() compute.Output {
	return compute.Output{NonDeterministic: true /* Because of the map */}
}

func (r *finishInvokeHandlers) Compute(ctx context.Context, deps compute.Resolved) (*handlerResult, error) {
	allOps := slices.Clone(r.definitions)

	perServer := map[schema.PackageName]*provisionOutput{}
	perServerOps := map[schema.PackageName][]*schema.SerializedInvocation{}

	ensure := func(pkg schema.PackageName) *provisionOutput {
		if existing, ok := perServer[pkg]; ok {
			return existing
		}
		perServer[pkg] = &provisionOutput{}
		return perServer[pkg]
	}

	for _, ext := range r.extensions {
		targetServer := ensure(schema.PackageName(ext.For))
		targetServer.Extensions = append(targetServer.Extensions, ext)
	}

	for _, ext := range r.serverExtensions {
		targetServer := ensure(schema.PackageName(ext.TargetServer))
		targetServer.ServerExtensions = append(targetServer.ServerExtensions, ext)
	}

	for k, handler := range r.handlers {
		s, ok := r.stack.Get(handler.TargetServer)
		if !ok {
			return nil, fnerrors.InternalError("found lifecycle for %q, but no such server in our stack", handler.TargetServer)
		}

		sr := ensure(handler.TargetServer)

		resp := compute.MustGetDepValue(deps, r.invocations[k], fmt.Sprintf("invocation%d", k))

		switch r.event {
		case protocol.Lifecycle_PROVISION:
			if resp.ApplyResponse.OutputResourceInstance != nil {
				return nil, fnerrors.InternalError("legacy provision tools can't produce outputs")
			}

			// XXX this needs revisiting as there's little to no isolation.
			// Probably lifecycle handlers should declare which servers they
			// apply to.
			for _, si := range resp.ApplyResponse.Extension {
				server, ok := r.stack.Get(schema.PackageName(si.For))
				if !ok {
					return nil, fnerrors.InternalError("%s: received startup input for %s, which is not in our stack",
						s.PackageName(), si.For)
				}

				if !handler.Source.Contains(server.PackageName()) {
					return nil, fnerrors.InternalError("%s: attempted to configure %q, which is not declared by the package",
						handler.Source.PackageName, si.For)
				}

				sr.Extensions = append(sr.Extensions, si)
			}

			for _, si := range resp.ApplyResponse.ServerExtension {
				server, ok := r.stack.Get(schema.PackageName(si.TargetServer))
				if !ok {
					return nil, fnerrors.InternalError("%s: received startup input for %s, which is not in our stack",
						s.PackageName(), si.TargetServer)
				}

				if !handler.Source.Contains(server.PackageName()) {
					return nil, fnerrors.InternalError("%s: attempted to configure %q, which is not declared by the package",
						handler.Source.PackageName, si.TargetServer)
				}

				if si.Owner != "" && si.Owner != handler.Source.PackageName.String() {
					return nil, fnerrors.BadInputError("%s: unexpected Owner %q", handler.Source.PackageName, si.Owner)
				}

				si.Owner = handler.Source.PackageName.String()
				sr.ServerExtensions = append(sr.ServerExtensions, si)
			}

			perServerOps[handler.TargetServer] = append(perServerOps[handler.TargetServer], resp.ApplyResponse.Invocation...)

			for _, src := range resp.ApplyResponse.InvocationSource {
				var computed []*schema.SerializedInvocation_ComputedValue

				// XXX make this extensible.

				for _, computable := range src.Computable {
					compiled, err := compileComputable(ctx, s.Server.SealedContext(), computable)
					if err != nil {
						return nil, err
					}

					serialized, err := anypb.New(compiled)
					if err != nil {
						return nil, err
					}

					computed = append(computed, &schema.SerializedInvocation_ComputedValue{
						Name:  computable.Name,
						Value: serialized,
					})
				}

				perServerOps[handler.TargetServer] = append(perServerOps[handler.TargetServer], &schema.SerializedInvocation{
					Description: src.Description,
					Scope:       src.Scope,
					Impl:        src.Impl,
					Computed:    computed,
				})
			}

			if len(resp.ApplyResponse.Computed) > 0 {
				if sr.ComputedConfigurations == nil {
					sr.ComputedConfigurations = &schema.ComputedConfigurations{}
				}

				if len(sr.ComputedConfigurations.Entry) == 0 {
					sr.ComputedConfigurations.Entry = append(sr.ComputedConfigurations.Entry, &schema.ComputedConfigurations_Entry{
						ServerPackage: handler.TargetServer.String(),
					})
				}

				sr.ComputedConfigurations.Entry[0].Configuration = append(sr.ComputedConfigurations.Entry[0].Configuration, resp.ApplyResponse.Computed...)
			}

		case protocol.Lifecycle_SHUTDOWN:
			perServerOps[handler.TargetServer] = append(perServerOps[handler.TargetServer], resp.DeleteResponse.Invocation...)
		}
	}

	for _, sr := range perServer {
		for _, computed := range sr.ComputedConfigurations.GetEntry() {
			slices.SortFunc(computed.Configuration, func(a, b *schema.ComputedConfiguration) bool {
				return strings.Compare(a.GetOwner(), b.GetOwner()) < 0
			})
		}
	}

	orderedOps, err := ensureInvocationOrder(ctx, r.stack, perServerOps)
	if err != nil {
		return nil, err
	}

	allOps = append(allOps, orderedOps...)

	return &handlerResult{Stack: r.stack, OrderedInvocations: allOps, ProvisionOutput: perServer}, nil
}

func sortServers(ctx context.Context, stack *planning.Stack) ([]schema.PackageName, error) {
	graph := toposort.NewGraph(0)

	// First ensure all nodes exist.
	for _, srv := range stack.Servers {
		fmt.Fprintf(console.Debug(ctx), "adding server: %s\n", srv.PackageName())
		if !graph.AddNode(srv.PackageName().String()) {
			return nil, fnerrors.InternalError("server %s appears multiple times in the stack", srv.PackageName())
		}
	}

	// Add edges for each server dep.
	for k, srv := range stack.Servers {
		for _, dep := range stack.Servers[k].ParsedDeps {
			for _, backend := range dep.ProvisionPlan.DeclaredStack {
				if backend == srv.PackageName() {
					return nil, fnerrors.InternalError("unexpected loop: %s depends on itself", srv.PackageName())
				}

				fmt.Fprintf(console.Debug(ctx), "adding edge: %s -> %s\n", backend, srv.PackageName())
				if !graph.AddEdge(backend.String(), srv.PackageName().String()) {
					return nil, fnerrors.InternalError("server dependency %s -> %s appears multiple times", backend, srv.PackageName())
				}
			}
		}
	}

	sortedServers, ok := graph.Toposort()
	if !ok {
		return nil, fnerrors.InternalError("failed to sort servers by dependency order")
	}

	fmt.Fprintf(console.Debug(ctx), "invocation sorted: %v\n", sortedServers)

	return schema.PackageNames(sortedServers...), nil
}

func ensureInvocationOrder(ctx context.Context, stack *planning.Stack, perServer map[schema.PackageName][]*schema.SerializedInvocation) ([]*schema.SerializedInvocation, error) {
	// We make sure that serialized invocations produced by a server A, that
	// depends on server B, are always run after B's serialized invocations.
	// This guarantees the pattern where B is a provider of an API -- and A is
	// the consumer, works. For example, B may create a CRD definition, and A
	// may instantiate that CRD.
	orderedServers, err := sortServers(ctx, stack)
	if err != nil {
		return nil, err
	}

	var allOps []*schema.SerializedInvocation
	for _, pkg := range orderedServers {
		allOps = append(allOps, perServer[schema.PackageName(pkg)]...)
	}

	return allOps, nil
}

func compileComputable(ctx context.Context, env pkggraph.SealedContext, src *schema.SerializedInvocationSource_ComputableValue) (proto.Message, error) {
	m, err := src.Value.UnmarshalNew()
	if err != nil {
		return nil, fnerrors.New("%s: failed to unmarshal: %w", src.Value.TypeUrl, err)
	}

	switch x := m.(type) {
	case *types.DeferredResource:
		if x.FromInvocation == nil {
			return nil, fnerrors.BadInputError("don't know how to compute resource")
		}

		compiledInvocation, prepared, err := makeInvocation(ctx, env, x.FromInvocation)
		if err != nil {
			return nil, err
		}

		var invocation compute.Computable[*protocol.InvokeResponse]
		switch {
		case compiledInvocation.ExperimentalFunction != nil:
			foundation, err := env.Resolve(ctx, "namespacelabs.dev/foundation")
			if err != nil {
				return nil, err
			}

			loc, err := env.Resolve(ctx, schema.PackageName(compiledInvocation.ExperimentalFunction.PackageName))
			if err != nil {
				return nil, err
			}

			invocation, err = tools.InvokeFunction(ctx, loc, foundation.Module.Abs(), compiledInvocation)
			if err != nil {
				return nil, err
			}

		case compiledInvocation.BinaryRef != nil:
			invocation, err = tools.InvokeWithBinary(ctx, env, compiledInvocation, prepared)
			if err != nil {
				return nil, err
			}

		default:
			return nil, fnerrors.BadInputError("don't know how to handle compiled invocation")
		}

		result, err := compute.GetValue(ctx, invocation)
		if err != nil {
			return nil, err
		}

		if result.Resource == nil {
			return nil, fnerrors.BadInputError("invocation didn't produce a resource")
		}

		return result.Resource, nil

	default:
		return nil, fnerrors.New("%s: don't know how to compile this type", src.Value.TypeUrl)
	}
}

func makeInvocation(ctx context.Context, env pkggraph.SealedContext, inv *types.DeferredInvocationSource) (*types.DeferredInvocation, *binary.Prepared, error) {
	if inv.ExperimentalFunction != "" {
		if inv.Binary != "" || inv.BinaryRef != nil {
			return nil, nil, fnerrors.New("binary and experimentalFunction are exclusive (%q vs %q)", inv.Binary, inv.ExperimentalFunction)
		}

		pkg, err := env.LoadByName(ctx, schema.PackageName(inv.ExperimentalFunction))
		if err != nil {
			return nil, nil, fnerrors.New("%s: failed to load: %w", inv.Binary, err)
		}

		if pkg.ExperimentalFunction == nil {
			return nil, nil, fnerrors.New("%s: missing function definition", inv.ExperimentalFunction)
		}

		return &types.DeferredInvocation{
			ExperimentalFunction: pkg.ExperimentalFunction,
			Cacheable:            inv.Cacheable,
			WithInput:            inv.WithInput,
		}, nil, nil
	}

	var ref *schema.PackageRef
	if inv.Binary != "" {
		// Legacy path, this should never be an implicit package reference.
		if strings.HasPrefix(inv.Binary, ":") {
			return nil, nil, fnerrors.InternalError("missing package name in reference %q", inv.Binary)
		}
		// Hack! Remove when we retire the legacy path.
		fakeOwner := schema.PackageName(env.Workspace().ModuleName())

		var err error
		ref, err = schema.ParsePackageRef(fakeOwner, inv.Binary)
		if err != nil {
			return nil, nil, fnerrors.New("%s: failed to parse package ref: %w", inv.Binary, err)
		}
	} else if inv.BinaryRef != nil {
		ref = inv.BinaryRef
	} else {
		return nil, nil, fnerrors.New("binary package definition is missing")
	}

	pkg, err := env.LoadByName(ctx, ref.AsPackageName())
	if err != nil {
		return nil, nil, fnerrors.New("%s: failed to load: %w", inv.Binary, err)
	}

	platform, err := tools.HostPlatform(ctx, env.Configuration())
	if err != nil {
		return nil, nil, err
	}

	prepared, err := binary.Plan(ctx, pkg, ref.Name, env, assets.AvailableBuildAssets{}, binary.BuildImageOpts{UsePrebuilts: true, Platforms: []specs.Platform{platform}})
	if err != nil {
		return nil, nil, err
	}

	return &types.DeferredInvocation{
		BinaryRef: ref,
		BinaryConfig: &schema.BinaryConfig{
			Command:    prepared.Command,
			Args:       prepared.Args,
			Env:        prepared.Env,
			WorkingDir: prepared.WorkingDir,
		},
		Cacheable: inv.Cacheable,
		WithInput: inv.WithInput,
	}, prepared, nil
}
