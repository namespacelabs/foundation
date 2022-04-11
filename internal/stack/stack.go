// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package stack

import (
	"context"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/eval"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Stack struct {
	Servers           []provision.Server
	Endpoints         []*schema.Endpoint
	InternalEndpoints []*schema.InternalEndpoint

	ParsedServers []*ParsedServer
}

type ProvisionOpts struct {
	PortBase int32
}

type ParsedServer struct {
	Deps []*ParsedNode
}

type ParsedNode struct {
	Package       *workspace.Package
	ProvisionPlan frontend.ProvisionPlan
	Allocations   []frontend.ValueWithPath
}

func (stack *Stack) Proto() *schema.Stack {
	s := &schema.Stack{
		Endpoint:         stack.Endpoints,
		InternalEndpoint: stack.InternalEndpoints,
	}

	for _, srv := range stack.Servers {
		s.Entry = append(s.Entry, srv.StackEntry())
	}

	return s
}

func (stack *Stack) Get(pkg schema.PackageName) *provision.Server {
	for _, s := range stack.Servers {
		if s.PackageName() == pkg {
			return &s
		}
	}
	return nil
}

// Implements `languages.Endpoints`.
func (stack *Stack) GetEndpoints() []*schema.Endpoint {
	return stack.Endpoints
}

func (stack *Stack) GetParsed(srv schema.PackageName) []*ParsedNode {
	for k, s := range stack.Servers {
		if s.PackageName() == srv {
			return stack.ParsedServers[k].Deps
		}
	}

	return nil
}

func Compute(ctx context.Context, servers []provision.Server, opts ProvisionOpts) (*Stack, error) {
	var s *Stack
	err := tasks.Action(runtime.TaskGraphCompute).Scope(provision.ServerPackages(servers).PackageNames()...).Run(ctx,
		func(ctx context.Context) error {
			var err error
			s, err = compute(ctx, opts, servers...)
			return err
		})

	return s, err
}

// XXX Unfortunately as we are today we need to pass provisioning information to stack computation
// because we need to yield definitions which have ports already materialized. Port allocation is
// more of a "startup" responsibility but this kept things simpler.
func compute(ctx context.Context, opts ProvisionOpts, servers ...provision.Server) (*Stack, error) {
	if len(servers) == 0 {
		return nil, fnerrors.InternalError("no server specified")
	}

	builder := newStackBuilder()

	ps := make([]*ParsedServer, len(servers))
	pkgs := make([]schema.PackageName, len(servers))
	for k, server := range servers {
		ps[k] = builder.Add(server)
		pkgs[k] = server.PackageName()
	}

	state := eval.NewAllocState()
	for k, server := range servers {
		if err := computeStackContents(ctx, server, ps[k], state, opts, builder); err != nil {
			return nil, err
		}
	}

	return builder.Seal(pkgs...), nil
}

func computeStackContents(ctx context.Context, server provision.Server, ps *ParsedServer, state *eval.AllocState, opts ProvisionOpts, out *stackBuilder) error {
	return tasks.Action("stack.compute").Scope(server.PackageName()).Run(ctx, func(ctx context.Context) error {
		deps := server.Deps()

		parsedDeps := make([]*ParsedNode, len(deps))
		exec, wait := executor.New(ctx)

		for k, n := range deps {
			k := k // Close k.
			n := n // Close n.
			loc := n.Location

			exec.Go(func(ctx context.Context) error {
				return tasks.Action("package.eval.provisioning").Scope(loc.PackageName).Run(ctx, func(ctx context.Context) error {
					ev, err := EvalProvision(ctx, server, n, state)
					if err != nil {
						return fnerrors.Wrap(loc, err)
					}

					parsedDeps[k] = ev

					for _, pkg := range ev.ProvisionPlan.DeclaredStack {
						pkg := pkg // Close pkg.

						exec.Go(func(ctx context.Context) error {
							server, ps, err := out.CheckAdd(ctx, server.Env(), pkg)
							if err != nil {
								return err
							}

							if ps == nil {
								// Already exists.
								return nil
							}

							return computeStackContents(ctx, *server, ps, state, opts, out)
						})
					}

					return nil
				})
			})
		}

		if err := wait(); err != nil {
			return err
		}

		var allocatedPorts eval.PortAllocations
		var allocators []eval.AllocatorFunc
		if opts.PortBase != 0 {
			allocators = append(allocators, eval.MakePortAllocator(opts.PortBase, &allocatedPorts))

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

			for _, dwn := range depsWithNeeds {
				allocs, err := fillNeeds(ctx, server.Proto(), state, allocators, dwn.Package.Node())
				if err != nil {
					return err
				}

				dwn.Allocations = allocs
			}
		}

		ps.Deps = parsedDeps

		// Fill in env-bound data now, post ports allocation.
		endpoints, internal, err := runtime.ComputeEndpoints(server.Env().Proto(), server.StackEntry(), allocatedPorts.Ports)
		if err != nil {
			return err
		}

		out.AddEndpoints(endpoints, internal)
		return err
	})
}

func EvalProvision(ctx context.Context, server provision.Server, n *workspace.Package, state *eval.AllocState) (*ParsedNode, error) {
	// We need to make sure that `env` is available before we read extend.stack, as env is often used
	// for branching.

	pdata, err := n.Parsed.EvalProvision(ctx, frontend.ProvisionInputs{
		Env:            server.Env().Proto(),
		Workspace:      server.Module().Workspace,
		ServerLocation: server.Location,
	})
	if err != nil {
		return nil, fnerrors.Wrap(n.Location, err)
	}

	if pdata.Naming != nil {
		return nil, fnerrors.UserError(n.Location, "nodes can't provide naming specifications")
	}

	if node := n.Node(); node != nil {
		if handler, ok := workspace.FrameworkHandlers[server.Framework()]; ok {
			fmwkData, err := handler.EvalProvision(node)
			if err != nil {
				return nil, err
			}
			pdata.DeclaredStack = append(pdata.DeclaredStack, fmwkData.DeclaredStack...)
		}
	}

	return &ParsedNode{Package: n, ProvisionPlan: pdata}, nil
}

func fillNeeds(ctx context.Context, server *schema.Server, s *eval.AllocState, allocators []eval.AllocatorFunc, n *schema.Node) ([]frontend.ValueWithPath, error) {
	var values []frontend.ValueWithPath
	for k := 0; k < len(n.GetNeed()); k++ {
		vwp, err := s.Alloc(ctx, server, allocators, n, k)
		if err != nil {
			return nil, err
		}
		values = append(values, vwp)
	}
	return values, nil
}
