// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

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
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision/tool"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/std/types"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func invokeHandlers(ctx context.Context, env planning.Context, stack *stack.Stack, handlers []*tool.Definition, event protocol.Lifecycle) (compute.Computable[*handlerResult], error) {
	props, err := runtime.PrepareProvision(ctx, env)
	if err != nil {
		return nil, err
	}

	propsPerServer := map[schema.PackageName]tool.InvokeProps{}

	definitions := props.Invocation
	extensions := props.Extension
	serverExtensions := props.ServerExtension

	for k, srv := range stack.ParsedServers {
		invokeProps := tool.InvokeProps{Event: event}

		invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, props.ProvisionInput...)

		for _, dep := range srv.Deps {
			// XXX breaks isolation.
			invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, dep.PrepareProps.ProvisionInput...)

			definitions = append(definitions, dep.PrepareProps.Invocations...)
			extensions = append(extensions, dep.PrepareProps.Extension...)
			serverExtensions = append(serverExtensions, dep.PrepareProps.ServerExtension...)
		}

		propsPerServer[stack.Servers[k].PackageName()] = invokeProps
	}

	var invocations []compute.Computable[*protocol.ToolResponse]
	for _, r := range handlers {
		focus := stack.Get(r.TargetServer)
		if focus == nil {
			return nil, fnerrors.InternalError("found lifecycle for %q, but no such server in our stack", r.TargetServer)
		}

		inv, err := tool.MakeInvocation(ctx, env, r, stack.Proto(), focus.PackageName(), propsPerServer[focus.PackageName()])
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

type handlerResult struct {
	Stack       *stack.Stack
	Definitions []*schema.SerializedInvocation
	Computed    *schema.ComputedConfigurations
	ServerDefs  map[schema.PackageName]*serverDefs // Per server.
}

type serverDefs struct {
	Server           schema.PackageName
	Ops              []*schema.SerializedInvocation
	ServerExtensions []*schema.ServerExtension
	Extensions       []*schema.DefExtension
	Computed         []*schema.ComputedConfiguration
}

type finishInvokeHandlers struct {
	stack            *stack.Stack
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
	allOps := append([]*schema.SerializedInvocation{}, r.definitions...)

	perServer := map[schema.PackageName]*serverDefs{}

	def := func(pkg schema.PackageName) *serverDefs {
		if existing, ok := perServer[pkg]; ok {
			return existing
		}
		perServer[pkg] = &serverDefs{Server: pkg}
		return perServer[pkg]
	}

	for _, ext := range r.extensions {
		targetServer := def(schema.PackageName(ext.For))
		targetServer.Extensions = append(targetServer.Extensions, ext)
	}

	for _, ext := range r.serverExtensions {
		targetServer := def(schema.PackageName(ext.TargetServer))
		targetServer.ServerExtensions = append(targetServer.ServerExtensions, ext)
	}

	for k, handler := range r.handlers {
		s := r.stack.Get(handler.TargetServer)
		if s == nil {
			return nil, fnerrors.InternalError("found lifecycle for %q, but no such server in our stack", handler.TargetServer)
		}

		sr := def(handler.TargetServer)

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

				sr.Extensions = append(sr.Extensions, si)
			}

			for _, si := range resp.ApplyResponse.ServerExtension {
				server := r.stack.Get(schema.PackageName(si.TargetServer))
				if server == nil {
					return nil, fnerrors.InternalError("%s: received startup input for %s, which is not in our stack",
						s.Location.PackageName, si.TargetServer)
				}

				if !handler.Source.Contains(server.PackageName()) {
					return nil, fnerrors.InternalError("%s: attempted to configure %q, which is not declared by the package",
						handler.Source.PackageName, si.TargetServer)
				}

				sr.ServerExtensions = append(sr.ServerExtensions, si)
			}

			sr.Ops = append(sr.Ops, resp.ApplyResponse.Invocation...)

			for _, src := range resp.ApplyResponse.InvocationSource {
				var computed []*schema.SerializedInvocation_ComputedValue

				// XXX make this extensible.

				for _, computable := range src.Computable {
					compiled, err := compileComputable(ctx, s.SealedContext(), computable)
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

				sr.Ops = append(sr.Ops, &schema.SerializedInvocation{
					Description: src.Description,
					Scope:       src.Scope,
					Impl:        src.Impl,
					Computed:    computed,
				})
			}

			sr.Computed = append(sr.Computed, resp.ApplyResponse.Computed...)

		case protocol.Lifecycle_SHUTDOWN:
			sr.Ops = append(sr.Ops, resp.DeleteResponse.Invocation...)
		}
	}

	computed := &schema.ComputedConfigurations{}
	for srv, sr := range perServer {
		slices.SortFunc(sr.Computed, func(a, b *schema.ComputedConfiguration) bool {
			return strings.Compare(a.GetOwner(), b.GetOwner()) < 0
		})

		computed.Entry = append(computed.Entry, &schema.ComputedConfigurations_Entry{
			ServerPackage: srv.String(),
			Configuration: sr.Computed,
		})
	}

	slices.SortFunc(computed.Entry, func(a, b *schema.ComputedConfigurations_Entry) bool {
		return strings.Compare(a.GetServerPackage(), b.GetServerPackage()) < 0
	})

	orderedOps, err := ensureInvocationOrder(ctx, r.stack, perServer)
	if err != nil {
		return nil, err
	}

	allOps = append(allOps, orderedOps...)

	return &handlerResult{r.stack, allOps, computed, perServer}, nil
}

const controllerPkg = "namespacelabs.dev/foundation/std/runtime/kubernetes/controller"

func ensureInvocationOrder(ctx context.Context, stack *stack.Stack, perServer map[schema.PackageName]*serverDefs) ([]*schema.SerializedInvocation, error) {
	// We make sure that serialized invocations produced by a server A, that
	// depends on server B, are always run after B's serialized invocations.
	// This guarantees the pattern where B is a provider of an API -- and A is
	// the consumer, works. For example, B may create a CRD definition, and A
	// may instantiate that CRD.
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
		for _, dep := range stack.ParsedServers[k].Deps {
			for _, backend := range dep.ProvisionPlan.DeclaredStack {
				if backend == srv.PackageName() {
					if srv.PackageName() == controllerPkg {
						// This is expected because the controller is added as a dependency to all servers (including itself).
						continue
					}
					return nil, fnerrors.InternalError("unexpected loop: %s depends on itself", srv.PackageName())
				}

				fmt.Fprintf(console.Debug(ctx), "adding edge: %s -> %s\n", backend, srv.PackageName())
				if !graph.AddEdge(backend.String(), srv.PackageName().String()) {
					return nil, fnerrors.InternalError("server dependency %s -> %s appears multiple times", backend, srv.PackageName())
				}
			}
		}
	}

	sorted, ok := graph.Toposort()
	if !ok {
		return nil, fnerrors.InternalError("failed to sort servers by dependency order")
	}

	fmt.Fprintf(console.Debug(ctx), "invocation sorted: %v\n", sorted)

	var allOps []*schema.SerializedInvocation
	for _, pkg := range sorted {
		if sr := perServer[schema.PackageName(pkg)]; sr != nil {
			allOps = append(allOps, sr.Ops...)
		}
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

	var binaryRef *schema.PackageRef
	if inv.Binary != "" {
		var err error
		binaryRef, err = schema.ParsePackageRef(inv.Binary)
		if err != nil {
			return nil, nil, fnerrors.New("%s: failed to parse package ref: %w", inv.Binary, err)
		}
	} else if inv.BinaryRef != nil {
		binaryRef = inv.BinaryRef
	} else {
		return nil, nil, fnerrors.New("binary package definition is missing")
	}

	pkg, err := env.LoadByName(ctx, binaryRef.PackageName())
	if err != nil {
		return nil, nil, fnerrors.New("%s: failed to load: %w", inv.Binary, err)
	}

	platform, err := tools.HostPlatform(ctx)
	if err != nil {
		return nil, nil, err
	}

	prepared, err := binary.Plan(ctx, pkg, binaryRef.Name, binary.BuildImageOpts{UsePrebuilts: true, Platforms: []specs.Platform{platform}})
	if err != nil {
		return nil, nil, err
	}

	return &types.DeferredInvocation{
		BinaryRef: binaryRef,
		BinaryConfig: &schema.BinaryConfig{
			Command: prepared.Command,
		},
		Cacheable: inv.Cacheable,
		WithInput: inv.WithInput,
	}, prepared, nil
}
