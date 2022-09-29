// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package provision

import (
	"context"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/frontend/invocation"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision/eval"
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Stack struct {
	Focus             schema.PackageList
	Servers           []Server
	Endpoints         []*schema.Endpoint
	InternalEndpoints []*schema.InternalEndpoint
}

type ProvisionOpts struct {
	PortRange runtime.PortRange
}

type Server struct {
	parsed.Server
	DeclaredStack schema.PackageList
	ParsedDeps    []*ParsedNode

	Endpoints         []*schema.Endpoint
	InternalEndpoints []*schema.InternalEndpoint
}

func (p Server) SidecarsAndInits() ([]*schema.SidecarContainer, []*schema.SidecarContainer) {
	var sidecars, inits []*schema.SidecarContainer

	sidecars = append(sidecars, p.Server.Provisioning.Sidecars...)
	inits = append(inits, p.Server.Provisioning.Inits...)

	for _, dep := range p.ParsedDeps {
		sidecars = append(sidecars, dep.ProvisionPlan.Sidecars...)
		inits = append(inits, dep.ProvisionPlan.Inits...)
	}

	return sidecars, inits
}

func (p Server) Integration() languages.Integration {
	return languages.IntegrationFor(p.Server.Framework())
}

type ParsedNode struct {
	Package       *pkggraph.Package
	ProvisionPlan pkggraph.ProvisionPlan
	Allocations   []pkggraph.ValueWithPath
	PrepareProps  struct {
		ProvisionInput  []*anypb.Any
		ServerExtension []*schema.ServerExtension
		Extension       []*schema.DefExtension
	}
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

func (stack *Stack) Get(srv schema.PackageName) (Server, bool) {
	for k, s := range stack.Servers {
		if s.PackageName() == srv {
			return stack.Servers[k], true
		}
	}

	return Server{}, false
}

func ComputeStack(ctx context.Context, servers parsed.Servers, opts ProvisionOpts) (*Stack, error) {
	return tasks.Return(ctx, tasks.Action("provision.Compute").Scope(servers.Packages().PackageNames()...),
		func(ctx context.Context) (*Stack, error) {
			return computeStack(ctx, opts, servers...)
		})
}

// XXX Unfortunately as we are today we need to pass provisioning information to stack computation
// because we need to yield definitions which have ports already materialized. Port allocation is
// more of a "startup" responsibility but this kept things simpler.
func computeStack(ctx context.Context, opts ProvisionOpts, servers ...parsed.Server) (*Stack, error) {
	if len(servers) == 0 {
		return nil, fnerrors.InternalError("no server specified")
	}

	var builder stackBuilder

	focus := make([]schema.PackageName, len(servers))
	for k, server := range servers {
		focus[k] = server.PackageName()
	}

	cs := computeState{exec: executor.New(ctx, "provision.Compute"), out: &builder}

	for _, srv := range servers {
		srv := srv // Close srv.

		cs.exec.Go(func(ctx context.Context) error {
			return cs.recursivelyComputeServerContents(ctx, srv.SealedContext(), srv.PackageName(), opts)
		})
	}

	if err := cs.exec.Wait(); err != nil {
		return nil, err
	}

	return builder.buildStack(focus...), nil
}

type computeState struct {
	exec *executor.Executor
	out  *stackBuilder
}

func (cs *computeState) recursivelyComputeServerContents(ctx context.Context, pkgs pkggraph.SealedContext, pkg schema.PackageName, opts ProvisionOpts) error {
	ps, existing := cs.out.claim(pkg)
	if existing {
		return nil // Already added.
	}

	srv, err := parsed.RequireLoadedServer(ctx, pkgs, pkg)
	if err != nil {
		return err
	}

	if err := computeServerContents(ctx, srv, opts, ps); err != nil {
		return err
	}

	for _, pkg := range ps.DeclaredStack.PackageNames() {
		pkg := pkg // Close pkg.
		cs.exec.Go(func(ctx context.Context) error {
			return cs.recursivelyComputeServerContents(ctx, pkgs, pkg, opts)
		})
	}

	return nil
}

func computeServerContents(ctx context.Context, server parsed.Server, opts ProvisionOpts, ps *Server) error {
	return tasks.Action("provision.evaluate").Scope(server.PackageName()).Run(ctx, func(ctx context.Context) error {
		deps := server.Deps()

		parsedDeps := make([]*ParsedNode, len(deps))
		exec := executor.New(ctx, "stack.provision.eval")

		for k, n := range deps {
			k := k    // Close k.
			node := n // Close n.

			exec.Go(func(ctx context.Context) error {
				ev, err := EvalProvision(ctx, server, node)
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

		// Fill in env-bound data now, post ports allocation.
		endpoints, internal, err := runtime.ComputeEndpoints(server, allocatedPorts.Ports)
		if err != nil {
			return err
		}

		ps.Endpoints = endpoints
		ps.InternalEndpoints = internal

		return err
	})
}

func EvalProvision(ctx context.Context, server parsed.Server, n *pkggraph.Package) (*ParsedNode, error) {
	return tasks.Return(ctx, tasks.Action("package.eval.provisioning").Scope(n.PackageName()).Arg("server", server.PackageName()), func(ctx context.Context) (*ParsedNode, error) {
		pn, err := evalProvision(ctx, server, n)
		if err != nil {
			return nil, fnerrors.Wrap(n.Location, err)
		}

		return pn, nil
	})
}

func evalProvision(ctx context.Context, server parsed.Server, node *pkggraph.Package) (*ParsedNode, error) {
	var combinedProps frontend.PrepareProps
	for _, hook := range node.PrepareHooks {
		if hook.InvokeInternal != "" {
			props, err := frontend.InvokeInternalPrepareHook(ctx, hook.InvokeInternal, server.SealedContext(), server.StackEntry())
			if err != nil {
				return nil, fnerrors.Wrap(node.Location, err)
			}

			if props == nil {
				continue
			}

			combinedProps.AppendWith(*props)
		} else if hook.InvokeBinary != nil {
			// XXX combine all builds beforehand.
			inv, err := invocation.Make(ctx, server.SealedContext(), nil, hook.InvokeBinary)
			if err != nil {
				return nil, err
			}

			if len(inv.Inject) > 0 {
				return nil, fnerrors.BadInputError("injection requested when it's not possible: %v", inv.Inject)
			}

			opts := rtypes.RunToolOpts{
				ImageName: inv.ImageName,
				RunBinaryOpts: rtypes.RunBinaryOpts{
					Command:    inv.Command,
					Args:       inv.Args,
					WorkingDir: inv.WorkingDir,
				},
				// XXX security prepare invocations have network access.
			}

			if (tools.CanConsumePublicImages(server.SealedContext().Configuration()) || tools.InvocationCanUseBuildkit) && inv.PublicImageID != nil {
				opts.PublicImageID = inv.PublicImageID
			} else {
				image, err := compute.GetValue(ctx, inv.Image)
				if err != nil {
					return nil, err
				}

				opts.Image = image
			}

			var invoke tools.LowLevelInvokeOptions[*protocol.PrepareRequest, *protocol.PrepareResponse]

			req := &protocol.PrepareRequest{
				Env:        server.SealedContext().Environment(),
				Server:     server.Proto(),
				ApiVersion: versions.APIVersion,
			}

			var resp *protocol.PrepareResponse

			if tools.InvocationCanUseBuildkit && opts.PublicImageID != nil {
				resp, err = invoke.InvokeOnBuildkit(ctx, server.SealedContext().Configuration(), "foundation.provision.tool.protocol.PrepareService/Prepare",
					node.PackageName(), *opts.PublicImageID, opts, req)
			} else {
				resp, err = invoke.Invoke(ctx, server.SealedContext().Configuration(), node.PackageName(), opts, req,
					func(conn *grpc.ClientConn) func(context.Context, *protocol.PrepareRequest, ...grpc.CallOption) (*protocol.PrepareResponse, error) {
						return protocol.NewPrepareServiceClient(conn).Prepare
					})
			}

			if err != nil {
				return nil, err
			}

			var pl schema.PackageList
			for _, p := range resp.GetPreparedProvisionPlan().GetDeclaredStack() {
				pl.Add(schema.PackageName(p))
			}

			combinedProps.AppendWith(frontend.PrepareProps{
				PreparedProvisionPlan: pkggraph.PreparedProvisionPlan{
					DeclaredStack:   pl.PackageNames(),
					ComputePlanWith: resp.GetPreparedProvisionPlan().GetProvisioning(),
					Sidecars:        resp.GetPreparedProvisionPlan().GetSidecar(),
					Inits:           resp.GetPreparedProvisionPlan().GetInit(),
				},
				ProvisionInput:  resp.ProvisionInput,
				Extension:       resp.Extension,
				ServerExtension: resp.ServerExtension,
			})
		}
	}

	// We need to make sure that `env` is available before we read extend.stack, as env is often used
	// for branching.
	pdata, err := node.Parsed.EvalProvision(ctx, server.SealedContext(), pkggraph.ProvisionInputs{
		ServerLocation: server.Location,
	})
	if err != nil {
		return nil, fnerrors.Wrap(node.Location, err)
	}

	if pdata.Naming != nil {
		return nil, fnerrors.UserError(node.Location, "nodes can't provide naming specifications")
	}

	for _, sidecar := range combinedProps.PreparedProvisionPlan.Sidecars {
		sidecar.Owner = node.PackageName().String()
	}

	for _, sidecar := range combinedProps.PreparedProvisionPlan.Inits {
		sidecar.Owner = node.PackageName().String()
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
