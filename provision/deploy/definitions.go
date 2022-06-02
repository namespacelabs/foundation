// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/tool"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func invokeHandlers(ctx context.Context, env ops.Environment, stack *stack.Stack, handlers []*tool.Definition, event protocol.Lifecycle) (compute.Computable[*handlerResult], error) {
	props, err := runtime.For(ctx, env).PrepareProvision(ctx)
	if err != nil {
		return nil, err
	}

	propsPerServer := map[schema.PackageName]tool.InvokeProps{}

	definitions := props.Invocation
	extensions := props.Extension

	for k, srv := range stack.ParsedServers {
		invokeProps := tool.InvokeProps{Event: event}

		invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, props.ProvisionInput...)

		for _, dep := range srv.Deps {
			invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, dep.PrepareProps.ProvisionInput...)

			definitions = append(definitions, dep.PrepareProps.Invocations...)
			extensions = append(extensions, dep.PrepareProps.Extension...)
		}

		propsPerServer[stack.Servers[k].PackageName()] = invokeProps
	}

	var invocations []compute.Computable[*protocol.ToolResponse]
	for _, r := range handlers {
		focus := stack.Get(r.For)
		if focus == nil {
			return nil, fnerrors.InternalError("found lifecycle for %q, but no such server in our stack", r.For)
		}

		invocations = append(invocations, tool.Invoke(ctx, env, r, stack.Proto(), focus.PackageName(), propsPerServer[focus.PackageName()]))
	}

	return &finishInvokeHandlers{
		stack:       stack,
		handlers:    handlers,
		invocations: invocations,
		event:       event,
		definitions: definitions,
		extensions:  extensions,
	}, nil
}

type handlerResult struct {
	Stack            *stack.Stack
	Definitions      []*schema.SerializedInvocation
	Computed         *schema.ComputedConfigurations
	ServerExtensions map[schema.PackageName][]*schema.DefExtension // Per server.
}

type finishInvokeHandlers struct {
	stack       *stack.Stack
	handlers    []*tool.Definition
	invocations []compute.Computable[*protocol.ToolResponse]
	event       protocol.Lifecycle
	definitions []*schema.SerializedInvocation
	extensions  []*schema.DefExtension

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
		JSON("extensions", r.extensions)
	for k, invocation := range r.invocations {
		in = in.Computable(fmt.Sprintf("invocation%d", k), invocation)
	}
	return in
}

func (r *finishInvokeHandlers) Output() compute.Output {
	return compute.Output{NonDeterministic: true /* Because of the map */}
}

func (r *finishInvokeHandlers) Compute(ctx context.Context, deps compute.Resolved) (*handlerResult, error) {
	ops := append([]*schema.SerializedInvocation{}, r.definitions...)

	extensionsPerServer := map[schema.PackageName][]*schema.DefExtension{}
	computedPerServer := map[schema.PackageName][]*schema.ComputedConfiguration{}

	for _, ext := range r.extensions {
		extensionsPerServer[schema.PackageName(ext.For)] = append(extensionsPerServer[schema.PackageName(ext.For)], ext)
	}

	for k, handler := range r.handlers {
		s := r.stack.Get(handler.For)
		if s == nil {
			return nil, fnerrors.InternalError("found lifecycle for %q, but no such server in our stack", handler.For)
		}

		resp := compute.MustGetDepValue(deps, r.invocations[k], fmt.Sprintf("invocation%d", k))

		switch r.event {
		case protocol.Lifecycle_PROVISION:
			// XXX this needs revisiting as there's little to no isolation.
			// Probably lifecycle handlers should declare which servers they
			// apply to.
			for _, si := range resp.ApplyResponse.Extension {
				server := r.stack.Get(schema.PackageName(si.For))
				if server == nil {
					return nil, fnerrors.InternalError("%s: received startup input for %s, which is not in our stack",
						s.Location.PackageName, si.For)
				}

				if !handler.Source.Contains(server.PackageName()) {
					return nil, fnerrors.InternalError("%s: attempted to configure %q, which is not declared by the package",
						handler.Source.PackageName, si.For)
				}

				extensionsPerServer[server.PackageName()] = append(extensionsPerServer[server.PackageName()], si)
			}

			ops = append(ops, resp.ApplyResponse.Invocation...)

			for _, src := range resp.ApplyResponse.InvocationSource {
				var computed []*schema.SerializedInvocation_ComputedValue

				// XXX make this extensible.

				for _, computable := range src.Computable {
					compiled, err := compileComputable(ctx, s.Env(), computable)
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

				ops = append(ops, &schema.SerializedInvocation{
					Description: src.Description,
					Scope:       src.Scope,
					Impl:        src.Impl,
					Computed:    computed,
				})
			}

			computedPerServer[schema.PackageName(handler.For)] = append(computedPerServer[schema.PackageName(handler.For)], resp.ApplyResponse.Computed...)

		case protocol.Lifecycle_SHUTDOWN:
			ops = append(ops, resp.DeleteResponse.Invocation...)
		}
	}

	computed := &schema.ComputedConfigurations{}
	for srv, configurations := range computedPerServer {
		computed.Entry = append(computed.Entry, &schema.ComputedConfigurations_Entry{
			ServerPackage: srv.String(),
			Configuration: configurations,
		})
	}

	slices.SortFunc(computed.Entry, func(a, b *schema.ComputedConfigurations_Entry) bool {
		return strings.Compare(a.GetServerPackage(), b.GetServerPackage()) < 0
	})

	return &handlerResult{r.stack, ops, computed, extensionsPerServer}, nil
}

func compileComputable(ctx context.Context, env provision.ServerEnv, src *schema.SerializedInvocationSource_ComputableValue) (proto.Message, error) {
	m, err := src.Value.UnmarshalNew()
	if err != nil {
		return nil, fnerrors.New("%s: failed to unmarshal: %w", src.Value.TypeUrl, err)
	}

	switch x := m.(type) {
	case *types.DeferredResourceSource:
		n := &types.DeferredResource{Inline: x.Inline}

		if x.FromInvocation != nil {
			compiled, err := makeInvocation(ctx, env, x.FromInvocation)
			if err != nil {
				return nil, err
			}

			n.FromInvocation = compiled
		}

		return n, nil

	case *types.DeferredInvocationSource:
		return makeInvocation(ctx, env, x)

	default:
		return nil, fnerrors.New("%s: don't know how to compile this type", src.Value.TypeUrl)
	}
}

func makeInvocation(ctx context.Context, env provision.ServerEnv, inv *types.DeferredInvocationSource) (*types.DeferredInvocation, error) {
	pkg, err := env.LoadByName(ctx, schema.PackageName(inv.Binary))
	if err != nil {
		return nil, fnerrors.New("%s: failed to load: %w", inv.Binary, err)
	}

	platform, err := tools.HostPlatform(ctx)
	if err != nil {
		return nil, err
	}

	prepared, err := binary.Plan(ctx, pkg, binary.BuildImageOpts{UsePrebuilts: true, Platforms: []specs.Platform{platform}})
	if err != nil {
		return nil, err
	}

	imageID, err := binary.EnsureImage(ctx, env, prepared)
	if err != nil {
		return nil, err
	}

	return &types.DeferredInvocation{
		BinaryPackage: inv.Binary,
		Image:         imageID.RepoAndDigest(),
		BinaryConfig: &schema.BinaryConfig{
			Command: prepared.Command,
		},
		Cacheable: inv.Cacheable,
		WithInput: inv.WithInput,
	}, nil
}
