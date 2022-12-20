// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import (
	"context"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/planning/invocation"
	"namespacelabs.dev/foundation/internal/planning/planninghooks"
	"namespacelabs.dev/foundation/internal/planning/secrets"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/runtime/tools"
	is "namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

type Stack struct {
	Focus             schema.PackageList
	Servers           []PlannedServer
	Endpoints         []*schema.Endpoint
	InternalEndpoints []*schema.InternalEndpoint
	ComputedResources map[string][]pkggraph.ResourceInstance // Key is resource ID.
}

type StackWithIngress struct {
	Stack
	IngressFragments []*schema.IngressFragment
}

func (s *StackWithIngress) GetIngressesForService(endpointOwner string, serviceName string) []*schema.IngressFragment {
	var result []*schema.IngressFragment

	for _, fragment := range s.IngressFragments {
		if fragment.GetOwner() != endpointOwner {
			continue
		}

		if fragment.GetEndpoint().GetServiceName() != serviceName {
			continue
		}

		result = append(result, fragment)
	}

	return result
}

type ProvisionOpts struct {
	Secrets   is.SecretsSource
	PortRange eval.PortRange
}

type PlannedServer struct {
	Server

	DeclaredStack schema.PackageList
	ParsedDeps    []*ParsedNode
	Resources     []pkggraph.ResourceInstance

	AllocatedPorts    []*schema.Endpoint_Port
	Endpoints         []*schema.Endpoint
	InternalEndpoints []*schema.InternalEndpoint
}

func (p PlannedServer) SidecarsAndInits() ([]*schema.Container, []*schema.Container) {
	var sidecars, inits []*schema.Container

	sidecars = append(sidecars, p.Server.Provisioning.Sidecars...)
	inits = append(inits, p.Server.Provisioning.Inits...)

	for _, dep := range p.ParsedDeps {
		sidecars = append(sidecars, dep.ProvisionPlan.Sidecars...)
		inits = append(inits, dep.ProvisionPlan.Inits...)
	}

	return sidecars, inits
}

type ParsedNode struct {
	Package       *pkggraph.Package
	ProvisionPlan pkggraph.ProvisionPlan
	Allocations   []pkggraph.ValueWithPath
	PrepareProps  planninghooks.ProvisionResult
}

func (stack *Stack) AllPackageList() schema.PackageList {
	var pl schema.PackageList
	for _, srv := range stack.Servers {
		pl.Add(srv.PackageName())
	}
	return pl
}

func (stack *Stack) Proto() *schema.Stack {
	s := &schema.Stack{
		Endpoint:         stack.Endpoints,
		InternalEndpoint: stack.InternalEndpoints,
	}

	for _, srv := range stack.Servers {
		s.Entry = append(s.Entry, srv.Server.StackEntry())
	}

	return s
}

func (stack *Stack) Get(srv schema.PackageName) (PlannedServer, bool) {
	for k, s := range stack.Servers {
		if s.PackageName() == srv {
			return stack.Servers[k], true
		}
	}

	return PlannedServer{}, false
}

func (stack *Stack) GetServerProto(srv schema.PackageName) (*schema.Server, bool) {
	server, ok := stack.Get(srv)
	if ok {
		return server.Proto(), true
	}

	return nil, false
}

func (stack *Stack) GetEndpoints(srv schema.PackageName) ([]*schema.Endpoint, bool) {
	server, ok := stack.Get(srv)
	if ok {
		return server.Endpoints, true
	}

	return nil, false
}

func (stack *Stack) GetComputedResources(resourceID string) []pkggraph.ResourceInstance {
	return stack.ComputedResources[resourceID]
}

func ComputeStack(ctx context.Context, servers Servers, opts ProvisionOpts) (*Stack, error) {
	return tasks.Return(ctx, tasks.Action("planning.compute").Scope(servers.Packages().PackageNames()...),
		func(ctx context.Context) (*Stack, error) {
			return computeStack(ctx, opts, servers...)
		})
}

// XXX Unfortunately as we are today we need to pass provisioning information to stack computation
// because we need to yield definitions which have ports already materialized. Port allocation is
// more of a "startup" responsibility but this kept things simpler.
func computeStack(ctx context.Context, opts ProvisionOpts, servers ...Server) (*Stack, error) {
	if len(servers) == 0 {
		return nil, fnerrors.InternalError("no server specified")
	}

	var builder stackBuilder

	focus := make([]schema.PackageName, len(servers))
	for k, server := range servers {
		focus[k] = server.PackageName()
	}

	eg := executor.New(ctx, "planning.compute")
	rp := newResourcePlanner(eg, opts.Secrets)
	cs := computeState{exec: eg, out: &builder}

	for _, srv := range servers {
		srv := srv // Close srv.

		cs.exec.Go(func(ctx context.Context) error {
			return cs.recursivelyComputeServerContents(ctx, rp, srv.SealedContext(), srv.PackageName(), opts)
		})
	}

	if err := cs.exec.Wait(); err != nil {
		return nil, err
	}

	return builder.buildStack(rp.Complete(), focus...), nil
}

type computeState struct {
	exec    *executor.Executor
	secrets is.SecretsSource
	out     *stackBuilder
}

func (cs *computeState) recursivelyComputeServerContents(ctx context.Context, rp *resourcePlanner, pkgs pkggraph.SealedContext, pkg schema.PackageName, opts ProvisionOpts) error {
	ps, existing := cs.out.claim(pkg)
	if existing {
		return nil // Already added.
	}

	srv, err := RequireLoadedServer(ctx, pkgs, pkg)
	if err != nil {
		return err
	}

	if err := cs.computeServerContents(ctx, rp, srv, opts, ps); err != nil {
		return err
	}

	for _, pkg := range ps.DeclaredStack.PackageNames() {
		pkg := pkg // Close pkg.
		cs.exec.Go(func(ctx context.Context) error {
			return cs.recursivelyComputeServerContents(ctx, rp, pkgs, pkg, opts)
		})
	}

	return nil
}

func (cs *computeState) computeServerContents(ctx context.Context, rp *resourcePlanner, server Server, opts ProvisionOpts, ps *PlannedServer) error {
	return tasks.Action("provision.evaluate").Scope(server.PackageName()).Run(ctx, func(ctx context.Context) error {
		deps := server.Deps()

		parsedDeps := make([]*ParsedNode, len(deps))
		exec := executor.New(ctx, "stack.provision.eval")

		for k, n := range deps {
			k := k    // Close k.
			node := n // Close n.

			exec.Go(func(ctx context.Context) error {
				ev, err := EvalProvision(ctx, cs.secrets, server, node)
				if err != nil {
					return err
				}

				parsedDeps[k] = ev
				return nil
			})
		}

		if err := exec.Wait(); err != nil {
			return err
		}

		var allocatedPorts eval.PortAllocations
		var allocators []eval.AllocatorFunc
		allocators = append(allocators, eval.MakePortAllocator(server.Proto(), opts.PortRange, &allocatedPorts))

		var depsWithNeeds []*ParsedNode
		for _, p := range parsedDeps {
			if len(p.Package.Node().GetNeed()) > 0 {
				depsWithNeeds = append(depsWithNeeds, p)
			}
		}

		// Make sure that port allocation is stable.
		sort.Slice(depsWithNeeds, func(i, j int) bool {
			return strings.Compare(depsWithNeeds[i].Package.PackageName().String(),
				depsWithNeeds[j].Package.PackageName().String()) < 0
		})

		state := eval.NewAllocState()
		for _, dwn := range depsWithNeeds {
			allocs, err := fillNeeds(ctx, server.Proto(), state, allocators, dwn.Package.Node())
			if err != nil {
				return err
			}

			dwn.Allocations = allocs
		}

		var declaredStack schema.PackageList
		declaredStack.AddMultiple(server.Provisioning.DeclaredStack...)
		for _, p := range parsedDeps {
			declaredStack.AddMultiple(p.ProvisionPlan.DeclaredStack...)
		}

		ps.Server = server
		ps.ParsedDeps = parsedDeps
		ps.DeclaredStack = declaredStack

		resources, err := parsing.LoadResources(ctx, server.SealedContext(), server.Package,
			server.PackageName().String(), server.Proto().GetResourcePack())
		if err != nil {
			return err
		}

		ps.Resources = append(ps.Resources, resources...)

		traverseResources(server.SealedContext(), server.PackageName().String(), rp, resources, func(pkg schema.PackageName) {
			cs.exec.Go(func(ctx context.Context) error {
				cs.out.changeServer(func() {
					ps.DeclaredStack.Add(pkg)
				})

				return cs.recursivelyComputeServerContents(ctx, rp, server.SealedContext(), pkg, opts)
			})
		})

		// Fill in env-bound data now, post ports allocation.
		endpoints, internal, err := ComputeEndpoints(server, allocatedPorts.Ports)
		if err != nil {
			return err
		}

		ps.AllocatedPorts = allocatedPorts.Ports
		ps.Endpoints = endpoints
		ps.InternalEndpoints = internal

		return err
	})
}

func traverseResources(sealedctx pkggraph.SealedContext, parentID string, r *resourcePlanner, instances []pkggraph.ResourceInstance, loadServer func(schema.PackageName)) error {
	var errs []error

	for _, res := range instances {
		if err := r.computeResource(sealedctx, parentID, res, loadServer); err != nil {
			errs = append(errs, err)
		}

		if err := traverseResources(sealedctx, res.ResourceID, r, res.Spec.ResourceInputs, loadServer); err != nil {
			errs = append(errs, err)
		}

		if res.Spec.Provider != nil {
			if err := traverseResources(sealedctx, res.ResourceID, r, res.Spec.Provider.Resources, loadServer); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return multierr.New(errs...)
}

func EvalProvision(ctx context.Context, secrets is.SecretsSource, server Server, n *pkggraph.Package) (*ParsedNode, error) {
	return tasks.Return(ctx, tasks.Action("package.eval.provisioning").Scope(n.PackageName()).Arg("server", server.PackageName()), func(ctx context.Context) (*ParsedNode, error) {
		pn, err := evalProvision(ctx, secrets, server, n)
		if err != nil {
			return nil, fnerrors.AttachLocation(n.Location, err)
		}

		return pn, nil
	})
}

func evalProvision(ctx context.Context, secs is.SecretsSource, server Server, node *pkggraph.Package) (*ParsedNode, error) {
	var combinedProps planninghooks.InternalPrepareProps
	for _, hook := range node.PrepareHooks {
		if hook.InvokeInternal != "" {
			props, err := planninghooks.InvokeInternalPrepareHook(ctx, hook.InvokeInternal, server.SealedContext(), server.StackEntry())
			if err != nil {
				return nil, fnerrors.AttachLocation(node.Location, err)
			}

			if props == nil {
				continue
			}

			combinedProps.AppendWith(*props)
		} else if hook.InvokeBinary != nil {
			// XXX combine all builds beforehand.
			inv, err := invocation.BuildAndPrepare(ctx, server.SealedContext(), server.SealedContext(), nil, hook.InvokeBinary)
			if err != nil {
				return nil, err
			}

			if len(inv.Inject) > 0 {
				return nil, fnerrors.BadInputError("injection requested when it's not possible: %v", inv.Inject)
			}

			opts := rtypes.RunBinaryOpts{
				Command:    inv.Config.Command,
				Args:       inv.Config.Args,
				WorkingDir: inv.Config.WorkingDir,
				Env:        inv.Config.Env,
				// XXX security prepare invocations have network access.
			}

			cli, err := buildkit.Client(ctx, server.SealedContext().Configuration(), nil)
			if err != nil {
				return nil, err
			}

			ximage := oci.ResolveImagePlatform(inv.Image, cli.BuildkitOpts().HostPlatform)

			req := &protocol.PrepareRequest{
				Env:        server.SealedContext().Environment(),
				Server:     server.Proto(),
				ApiVersion: int32(versions.Builtin().APIVersion),
			}

			invocation := tools.InvokeOnBuildkit[*protocol.PrepareResponse](
				cli, secrets.ScopeSecretsTo(secs, server.SealedContext(), nil),
				"foundation.provision.tool.protocol.PrepareService/Prepare",
				node.PackageName(), ximage, opts, req, tools.LowLevelInvokeOptions{})
			resp, err := compute.GetValue(ctx, invocation)
			if err != nil {
				return nil, err
			}

			var pl schema.PackageList
			for _, p := range resp.GetPreparedProvisionPlan().GetDeclaredStack() {
				pl.Add(schema.PackageName(p))
			}

			if len(resp.DeprecatedProvisionInput) > 0 {
				return nil, fnerrors.BadInputError("setting provision inputs is deprecated, use serialized message")
			}

			props := planninghooks.InternalPrepareProps{
				PreparedProvisionPlan: pkggraph.PreparedProvisionPlan{
					DeclaredStack:   pl.PackageNames(),
					ComputePlanWith: resp.GetPreparedProvisionPlan().GetProvisioning(),
					Sidecars:        resp.GetPreparedProvisionPlan().GetSidecar(),
					Inits:           resp.GetPreparedProvisionPlan().GetInit(),
				},
				ProvisionResult: planninghooks.ProvisionResult{
					SerializedProvisionInput: resp.ProvisionInput,
					Extension:                resp.Extension,
					ServerExtension:          resp.ServerExtension,
				},
			}

			combinedProps.AppendWith(props)
		}
	}

	// We need to make sure that `env` is available before we read extend.stack, as env is often used
	// for branching.
	pdata, err := node.Parsed.EvalProvision(ctx, server.SealedContext(), pkggraph.ProvisionInputs{
		ServerLocation: server.Location,
	})
	if err != nil {
		return nil, fnerrors.AttachLocation(node.Location, err)
	}

	if pdata.Naming != nil {
		return nil, fnerrors.NewWithLocation(node.Location, "nodes can't provide naming specifications")
	}

	for _, sidecar := range combinedProps.PreparedProvisionPlan.Sidecars {
		sidecar.Owner = schema.MakePackageSingleRef(node.PackageName())
	}

	for _, sidecar := range combinedProps.PreparedProvisionPlan.Inits {
		sidecar.Owner = schema.MakePackageSingleRef(node.PackageName())
	}

	pdata.AppendWith(combinedProps.PreparedProvisionPlan)

	parsed := &ParsedNode{Package: node, ProvisionPlan: pdata}
	parsed.PrepareProps.ProvisionInput = combinedProps.ProvisionInput
	parsed.PrepareProps.Extension = combinedProps.Extension
	parsed.PrepareProps.ServerExtension = combinedProps.ServerExtension

	return parsed, nil
}

func fillNeeds(ctx context.Context, server *schema.Server, s *eval.AllocState, allocators []eval.AllocatorFunc, n *schema.Node) ([]pkggraph.ValueWithPath, error) {
	var values []pkggraph.ValueWithPath
	for k := 0; k < len(n.GetNeed()); k++ {
		vwp, err := s.Alloc(ctx, server, allocators, n, k)
		if err != nil {
			return nil, err
		}
		values = append(values, vwp)
	}
	return values, nil
}
