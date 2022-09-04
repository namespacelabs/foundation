// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package stack

import (
	"context"
	"sort"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/frontend/invocation"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/eval"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type Stack struct {
	Servers           []provision.Server
	Endpoints         []*schema.Endpoint
	InternalEndpoints []*schema.InternalEndpoint

	ParsedServers []*ParsedServer
}

type ProvisionOpts struct {
	PortRange runtime.PortRange
}

type ParsedServer struct {
	DeclaredStack               schema.PackageList
	Deps                        []*ParsedNode
	ServerSidecars, ServerInits []*schema.SidecarContainer
}

func (p ParsedServer) SidecarsAndInits() ([]*schema.SidecarContainer, []*schema.SidecarContainer) {
	var sidecars, inits []*schema.SidecarContainer

	sidecars = append(sidecars, p.ServerSidecars...)
	inits = append(inits, p.ServerInits...)

	for _, dep := range p.Deps {
		sidecars = append(sidecars, dep.ProvisionPlan.Sidecars...)
		inits = append(inits, dep.ProvisionPlan.Inits...)
	}

	return sidecars, inits
}

type ParsedNode struct {
	Package       *workspace.Package
	ProvisionPlan pkggraph.ProvisionPlan
	Allocations   []pkggraph.ValueWithPath
	PrepareProps  struct {
		ProvisionInput  []*anypb.Any
		Invocations     []*schema.SerializedInvocation
		ServerExtension []*schema.ServerExtension
		Extension       []*schema.DefExtension
	}
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

func (stack *Stack) GetParsed(srv schema.PackageName) *ParsedServer {
	for k, s := range stack.Servers {
		if s.PackageName() == srv {
			return stack.ParsedServers[k]
		}
	}

	return nil
}

func Compute(ctx context.Context, servers []provision.Server, opts ProvisionOpts) (*Stack, error) {
	return tasks.Return(ctx, tasks.Action(runtime.TaskStackCompute).Scope(provision.ServerPackages(servers).PackageNames()...),
		func(ctx context.Context) (*Stack, error) {
			return computeStack(ctx, opts, servers...)
		})
}

// XXX Unfortunately as we are today we need to pass provisioning information to stack computation
// because we need to yield definitions which have ports already materialized. Port allocation is
// more of a "startup" responsibility but this kept things simpler.
func computeStack(ctx context.Context, opts ProvisionOpts, servers ...provision.Server) (*Stack, error) {
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

	cs := computeState{exec: executor.New(ctx, "stack.compute"), out: builder, opts: opts}

	for k := range servers {
		k := k // Close k.

		cs.exec.Go(func(ctx context.Context) error {
			return cs.computeStackContents(ctx, servers[k], ps[k])
		})
	}

	if err := cs.exec.Wait(); err != nil {
		return nil, err
	}

	return builder.Seal(pkgs...), nil
}

type computeState struct {
	exec *executor.Executor
	out  *stackBuilder
	opts ProvisionOpts
}

func (cs *computeState) checkAdd(env pkggraph.SealedContext, pkg schema.PackageName) {
	cs.exec.Go(func(ctx context.Context) error {
		server, ps, err := cs.out.checkAdd(ctx, env, pkg)
		if err != nil {
			return err
		}

		if ps == nil {
			// Already exists.
			return nil
		}

		return cs.computeStackContents(ctx, *server, ps)
	})
}

func (cs *computeState) computeStackContents(ctx context.Context, server provision.Server, ps *ParsedServer) error {
	return tasks.Action("provision.evaluate").Scope(server.PackageName()).Run(ctx, func(ctx context.Context) error {
		deps := server.Deps()

		parsedDeps := make([]*ParsedNode, len(deps))
		exec := executor.New(ctx, "stack.provision.eval")

		for k, n := range deps {
			k := k // Close k.
			n := n // Close n.

			exec.Go(func(ctx context.Context) error {
				ev, err := EvalProvision(ctx, server, n)
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

		var declaredStack schema.PackageList
		declaredStack.AddMultiple(server.Provisioning.DeclaredStack...)
		for _, p := range parsedDeps {
			declaredStack.AddMultiple(p.ProvisionPlan.DeclaredStack...)
		}

		for _, p := range declaredStack.PackageNames() {
			cs.checkAdd(server.SealedContext(), p)
		}

		var allocatedPorts eval.PortAllocations
		var allocators []eval.AllocatorFunc
		allocators = append(allocators, eval.MakePortAllocator(server.Proto(), cs.opts.PortRange, &allocatedPorts))

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

		ps.Deps = parsedDeps
		ps.ServerSidecars = server.Provisioning.Sidecars
		ps.ServerInits = server.Provisioning.Inits
		ps.DeclaredStack = declaredStack

		// Fill in env-bound data now, post ports allocation.
		endpoints, internal, err := runtime.ComputeEndpoints(server, allocatedPorts.Ports)
		if err != nil {
			return err
		}

		cs.out.AddEndpoints(endpoints, internal)
		return err
	})
}

func EvalProvision(ctx context.Context, server provision.Server, n *workspace.Package) (*ParsedNode, error) {
	return tasks.Return(ctx, tasks.Action("package.eval.provisioning").Scope(n.PackageName()).Arg("server", server.PackageName()), func(ctx context.Context) (*ParsedNode, error) {
		pn, err := evalProvision(ctx, server, n)
		if err != nil {
			return nil, fnerrors.Wrap(n.Location, err)
		}

		return pn, nil
	})
}

func evalProvision(ctx context.Context, server provision.Server, n *workspace.Package) (*ParsedNode, error) {
	var combinedProps frontend.PrepareProps
	for _, hook := range n.PrepareHooks {
		if hook.InvokeInternal != "" {
			props, err := frontend.InvokeInternalPrepareHook(ctx, hook.InvokeInternal, server.SealedContext(), server.StackEntry())
			if err != nil {
				return nil, fnerrors.Wrap(n.Location, err)
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

			if (tools.CanConsumePublicImages() || tools.InvocationCanUseBuildkit) && inv.PublicImageID != nil {
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
				Env:    server.SealedContext().Environment(),
				Server: server.Proto(),
			}

			var resp *protocol.PrepareResponse

			if tools.InvocationCanUseBuildkit && opts.PublicImageID != nil {
				resp, err = invoke.BuildkitInvocation(ctx, server.SealedContext(), "foundation.provision.tool.protocol.PrepareService/Prepare",
					schema.PackageName(hook.InvokeBinary.Binary), *opts.PublicImageID, opts, req)
			} else {
				resp, err = invoke.Invoke(ctx, n.PackageName(), opts, req, func(conn *grpc.ClientConn) func(context.Context, *protocol.PrepareRequest, ...grpc.CallOption) (*protocol.PrepareResponse, error) {
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
					ProvisionStack: pkggraph.ProvisionStack{
						DeclaredStack: pl.PackageNames(),
					},
					Provisioning: resp.GetPreparedProvisionPlan().GetProvisioning(),
					Sidecars:     resp.GetPreparedProvisionPlan().GetSidecar(),
					Inits:        resp.GetPreparedProvisionPlan().GetInit(),
				},
				ProvisionInput:  resp.ProvisionInput,
				Invocations:     resp.Invocation,
				Extension:       resp.Extension,
				ServerExtension: resp.ServerExtension,
			})
		}
	}

	// We need to make sure that `env` is available before we read extend.stack, as env is often used
	// for branching.
	pdata, err := n.Parsed.EvalProvision(ctx, server.SealedContext(), pkggraph.ProvisionInputs{
		Workspace:      server.Module().Workspace,
		ServerLocation: server.Location,
	})
	if err != nil {
		return nil, fnerrors.Wrap(n.Location, err)
	}

	if pdata.Naming != nil {
		return nil, fnerrors.UserError(n.Location, "nodes can't provide naming specifications")
	}

	pdata.AppendWith(combinedProps.PreparedProvisionPlan)

	node := &ParsedNode{Package: n, ProvisionPlan: pdata}
	node.PrepareProps.ProvisionInput = combinedProps.ProvisionInput
	node.PrepareProps.Invocations = combinedProps.Invocations
	node.PrepareProps.Extension = combinedProps.Extension
	node.PrepareProps.ServerExtension = combinedProps.ServerExtension

	return node, nil
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
