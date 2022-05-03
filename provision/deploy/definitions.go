// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision/tool"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func invokeHandlers(ctx context.Context, env ops.Environment, stack *stack.Stack, handlers []*tool.Definition, event protocol.Lifecycle) (compute.Computable[*handlerResult], error) {
	props, err := runtime.For(ctx, env).PrepareProvision(ctx)
	if err != nil {
		return nil, err
	}

	propsPerServer := map[schema.PackageName]tool.InvokeProps{}

	definitions := props.Definition
	extensions := props.Extension

	for k, srv := range stack.ParsedServers {
		invokeProps := tool.InvokeProps{Event: event}

		invokeProps.LocalMapping = append(invokeProps.LocalMapping, props.LocalMapping...)
		invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, props.ProvisionInput...)

		for _, dep := range srv.Deps {
			invokeProps.ProvisionInput = append(invokeProps.ProvisionInput, dep.PrepareProps.ProvisionInput...)

			definitions = append(definitions, dep.PrepareProps.Definition...)
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
	Definitions      []*schema.Definition
	ServerExtensions map[schema.PackageName][]*schema.DefExtension
}

type finishInvokeHandlers struct {
	stack       *stack.Stack
	handlers    []*tool.Definition
	invocations []compute.Computable[*protocol.ToolResponse]
	event       protocol.Lifecycle
	definitions []*schema.Definition
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
	ops := append([]*schema.Definition{}, r.definitions...)

	extensionsPerServer := map[schema.PackageName][]*schema.DefExtension{}

	for _, ext := range r.extensions {
		extensionsPerServer[schema.PackageName(ext.For)] = append(extensionsPerServer[schema.PackageName(ext.For)], ext)
	}

	for k, handler := range r.handlers {
		s := r.stack.Get(handler.For)
		if s == nil {
			return nil, fnerrors.InternalError("found lifecycle for %q, but no such server in our stack", handler.For)
		}

		resp := compute.GetDepValue(deps, r.invocations[k], fmt.Sprintf("invocation%d", k))

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

			ops = append(ops, resp.ApplyResponse.Definition...)

		case protocol.Lifecycle_SHUTDOWN:
			ops = append(ops, resp.DeleteResponse.Definition...)
		}
	}

	return &handlerResult{r.stack, ops, extensionsPerServer}, nil
}
