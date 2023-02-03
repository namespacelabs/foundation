// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/planning/invocation"
	"namespacelabs.dev/foundation/internal/planning/planninghooks"
	"namespacelabs.dev/foundation/internal/planning/secrets"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/runtime/tools"
	is "namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/internal/support"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/library/kubernetes/ingress"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/foundation/std/runtime/constants"
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

	// Transitive set of servers this server depends on.
	DeclaredStack schema.PackageList
	// Post-processed set of `node` dependencies.
	ParsedDeps     []*ParsedNode
	Resources      []pkggraph.ResourceInstance
	MergedFragment *schema.ServerFragment

	AllocatedPorts    []*schema.Endpoint_Port
	Endpoints         []*schema.Endpoint
	InternalEndpoints []*schema.InternalEndpoint
}

type ParsedNode struct {
	Package         *pkggraph.Package
	ServerFragments []*schema.ServerFragment
	Startup         pkggraph.PreStartup
	ComputePlanWith []*schema.Invocation
	Allocations     []pkggraph.ValueWithPath
	PrepareProps    planninghooks.ProvisionResult
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

func (stack *Stack) GetStackEntry(srv schema.PackageName) (*schema.Stack_Entry, bool) {
	server, ok := stack.Get(srv)
	if ok {
		return server.entry, true
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

		ps.Server = server
		ps.ParsedDeps = parsedDeps

		allFragments := server.fragments
		for _, n := range parsedDeps {
			allFragments = append(allFragments, n.ServerFragments...)
		}

		if len(allFragments) == 0 {
			return fnerrors.InternalError("list of server fragments should never be empty")
		}

		ps.MergedFragment = protos.Clone(allFragments[0])
		ps.MergedFragment.MainContainer.Owner = nil

		var extensionList schema.PackageList
		extensionList.AddMultiple(schema.PackageNames(ps.MergedFragment.Extension...)...)

		for i := 1; i < len(allFragments); i++ {
			frag := allFragments[i]

			if frag.MainContainer.BinaryRef != nil {
				if ps.MergedFragment.MainContainer.BinaryRef != nil {
					return fnerrors.New("main_container.binary_ref set more than once")
				}
				ps.MergedFragment.MainContainer.BinaryRef = frag.MainContainer.BinaryRef
			}

			if frag.MainContainer.Name != "" {
				if ps.MergedFragment.MainContainer.Name != "" {
					return fnerrors.New("main_container.Name set more than once")
				}
				ps.MergedFragment.MainContainer.Name = frag.MainContainer.Name
			}

			ps.MergedFragment.MainContainer.Args = append(ps.MergedFragment.MainContainer.Args, frag.MainContainer.Args...)
			ps.MergedFragment.MainContainer.Mount = append(ps.MergedFragment.MainContainer.Mount, frag.MainContainer.Mount...)

			var err error
			ps.MergedFragment.MainContainer.Env, err = support.MergeEnvs(ps.MergedFragment.MainContainer.Env, frag.MainContainer.Env)
			if err != nil {
				return err
			}

			if frag.MainContainer.Security != nil {
				if ps.MergedFragment.MainContainer.Security != nil {
					return fnerrors.New("main_container.security set more than once")
				}
				ps.MergedFragment.MainContainer.Security = frag.MainContainer.Security
			}

			ps.MergedFragment.Sidecar = append(ps.MergedFragment.Sidecar, frag.Sidecar...)
			ps.MergedFragment.InitContainer = append(ps.MergedFragment.InitContainer, frag.InitContainer...)

			ps.MergedFragment.Service = append(ps.MergedFragment.Service, frag.Service...)
			ps.MergedFragment.Ingress = append(ps.MergedFragment.Ingress, frag.Ingress...)
			ps.MergedFragment.Probe = append(ps.MergedFragment.Probe, frag.Probe...)
			// XXX dedup
			ps.MergedFragment.Volume = append(ps.MergedFragment.Volume, frag.Volume...)
			ps.MergedFragment.Toleration = append(ps.MergedFragment.Toleration, frag.Toleration...)

			if frag.Permissions != nil {
				if ps.MergedFragment.Permissions == nil {
					ps.MergedFragment.Permissions = &schema.ServerPermissions{}
				}
				ps.MergedFragment.Permissions.ClusterRole = append(ps.MergedFragment.Permissions.ClusterRole, frag.Permissions.ClusterRole...)
			}

			if frag.ResourcePack != nil {
				if ps.MergedFragment.ResourcePack == nil {
					ps.MergedFragment.ResourcePack = &schema.ResourcePack{}
				}
				parsing.MergeResourcePack(frag.ResourcePack, ps.MergedFragment.ResourcePack)
			}

			extensionList.AddMultiple(schema.PackageNames(frag.Extension...)...)
		}

		// XXX NSL-533
		// for _, vol := range ps.MergedFragment.Volume {
		// 	if vol.Kind == constants.VolumeKindPersistent {
		// 		if server.Proto().DeployableClass != string(schema.DeployableClass_STATEFUL) {
		// 			return fnerrors.BadInputError("%s: servers that use persistent storage are required to be of class %q",
		// 				server.PackageName(), schema.DeployableClass_STATEFUL)
		// 		}
		// 		break
		// 	}
		// }

		if err := validateServer(ctx, server.Location, ps.MergedFragment); err != nil {
			return err
		}

		ps.MergedFragment.Extension = extensionList.PackageNamesAsString()

		computed, err := parsing.LoadResources(ctx, server.SealedContext(), server.Package,
			server.PackageName().String(), ps.MergedFragment.ResourcePack)
		if err != nil {
			return err
		}

		ps.Resources = append(ps.Resources, computed...)

		// Fill in env-bound data now, post ports allocation.
		endpoints, internal, err := ComputeEndpoints(server, ps.MergedFragment, allocatedPorts.Ports)
		if err != nil {
			return err
		}

		var ignored []string
		ingressEndpoints := map[string][]*schema.Endpoint{}
		for _, endpoint := range endpoints {
			if endpoint.Type == schema.Endpoint_INTERNET_FACING {
				if endpoint.IngressProvider == nil {
					ignored = append(ignored, endpoint.ServiceName)
				} else {
					ingressEndpoints[endpoint.IngressProvider.Canonical()] = append(ingressEndpoints[endpoint.IngressProvider.Canonical()], endpoint)
				}
			}
		}

		if len(ignored) > 0 {
			fmt.Fprintf(console.Debug(ctx), "Ignored the following services in %q, due to a lack of ingress provider: %s\n",
				server.PackageName(), strings.Join(ignored, ", "))
		}

		if len(ingressEndpoints) > 0 {
			if err := pkggraph.ValidateFoundation("computed ingress", 56, pkggraph.ModuleFromModules(server.SealedContext())); err != nil {
				return err
			}

			ingressClassRef := &schema.PackageRef{
				PackageName: "namespacelabs.dev/foundation/library/runtime",
				Name:        "Ingress",
			}

			ingressClass, err := pkggraph.LookupResourceClass(ctx, server.SealedContext(), server.Package, ingressClassRef)
			if err != nil {
				return err
			}

			ingressProviders := maps.Keys(ingressEndpoints)
			slices.Sort(ingressProviders)
			for k, providerRef := range ingressProviders {
				provider, err := parsing.LookupResourceProvider(ctx, server.SealedContext(), server.Package, providerRef, ingressClassRef)
				if err != nil {
					return err
				}

				if provider.Spec.PrepareWith == nil || provider.Spec.InitializedWith != nil {
					return fnerrors.InternalError("for the time being, ingress providers must operate in the planning phase (i.e. must set prepareWith)")
				}

				endpoints := ingressEndpoints[providerRef]

				ref := &schema.PackageRef{
					PackageName: server.PackageName().String(),
					Name:        fmt.Sprintf("%s$ingress_%d", server.Proto().Name, k),
				}

				var applicationDomains []string
				if server.env.Environment().Purpose == schema.Environment_DEVELOPMENT {
					applicationDomains = append(applicationDomains, "nslocal.host")
				}

				intent := &ingress.IngressIntent{
					Env:                   protos.Clone(server.env.Environment()),
					Endpoint:              endpoints,
					Deployable:            runtime.DeployableToProto(server.Proto()),
					ApplicationBaseDomain: applicationDomains,
				}

				if intent.Env.Ephemeral {
					intent.Env.Name = "" // Make sure we don't trash the cache in tests.
				}

				boxedIntent, err := anypb.New(intent)
				if err != nil {
					return fnerrors.InternalError("failed to serialize ingress intent: %w", err)
				}

				intentJSON, err := json.Marshal(intent)
				if err != nil {
					return fnerrors.InternalError("failed to json serialize ingress intent: %w", err)
				}

				ps.Resources = append(ps.Resources, pkggraph.ResourceInstance{
					ResourceID:  resources.ResourceID(ref),
					ResourceRef: ref,
					Spec: pkggraph.ResourceSpec{
						Source: &schema.ResourceInstance{
							PackageName:          ref.PackageName,
							Name:                 ref.Name,
							Class:                ingressClassRef,
							SerializedIntentJson: string(intentJSON),
						},
						Intent:     boxedIntent,
						IntentType: provider.IntentType,
						Class:      *ingressClass,
						Provider:   provider,
					},
				})
			}
		}

		traverseResources(server.SealedContext(), server.PackageName().String(), rp, ps.Resources, func(pkg schema.PackageName) {
			cs.exec.Go(func(ctx context.Context) error {
				cs.out.changeServer(func() {
					ps.DeclaredStack.Add(pkg)
				})

				return cs.recursivelyComputeServerContents(ctx, rp, server.SealedContext(), pkg, opts)
			})
		})

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

func validateServer(ctx context.Context, loc pkggraph.Location, srv *schema.ServerFragment) error {
	filesyncControllerMounts := 0
	for _, m := range srv.GetMainContainer().Mount {
		// Only supporting volumes within the same package for now.
		volume := findVolume(srv.GetVolume(), m.VolumeRef)
		if volume == nil {
			return fnerrors.NewWithLocation(loc, "volume %q does not exist", m.VolumeRef.Canonical())
		}
		if volume.Kind == constants.VolumeKindWorkspaceSync {
			filesyncControllerMounts++
		}
	}
	if filesyncControllerMounts > 1 {
		return fnerrors.NewWithLocation(loc, "only one workspace sync mount is allowed per server")
	}

	volumeNames := map[string]struct{}{}
	for _, v := range srv.GetVolume() {
		if _, ok := volumeNames[v.Name]; ok {
			return fnerrors.NewWithLocation(loc, "volume %q is defined multiple times", v.Name)
		}
		volumeNames[v.Name] = struct{}{}
	}

	return nil
}

func findVolume(volumes []*schema.Volume, ref *schema.PackageRef) *schema.Volume {
	for _, v := range volumes {
		if v.Owner == ref.PackageName && v.Name == ref.Name {
			return v
		}
	}
	return nil
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
	var fragments []*schema.ServerFragment
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

			if len(resp.DeprecatedProvisionInput) > 0 {
				return nil, fnerrors.BadInputError("setting provision inputs is deprecated, use serialized message")
			}

			props := planninghooks.InternalPrepareProps{
				ComputePlanWith: resp.GetPreparedProvisionPlan().GetProvisioning(),
				ProvisionResult: planninghooks.ProvisionResult{
					SerializedProvisionInput: resp.ProvisionInput,
					Extension:                resp.Extension,
					ServerExtension:          resp.ServerExtension,
				},
			}

			if len(resp.GetPreparedProvisionPlan().GetSidecar()) > 0 || len(resp.GetPreparedProvisionPlan().GetInit()) > 0 || len(resp.GetPreparedProvisionPlan().GetDeclaredStack()) > 0 {
				frag := &schema.ServerFragment{
					MainContainer: &schema.Container{},
				}

				for _, sidecar := range resp.GetPreparedProvisionPlan().GetSidecar() {
					sidecar.Owner = schema.MakePackageSingleRef(node.PackageName())
					frag.Sidecar = append(frag.Sidecar, sidecar)
				}

				for _, init := range resp.GetPreparedProvisionPlan().GetInit() {
					init.Owner = schema.MakePackageSingleRef(node.PackageName())
					frag.InitContainer = append(frag.InitContainer, init)
				}

				if len(resp.GetPreparedProvisionPlan().GetDeclaredStack()) > 0 {
					if err := parsing.AddServersAsResources(ctx, server.SealedContext(), server.PackageRef(),
						schema.PackageNames(resp.GetPreparedProvisionPlan().GetDeclaredStack()...), frag); err != nil {
						return nil, err
					}
				}

				fragments = append(fragments, frag)
			}

			combinedProps.AppendWith(props)
		}
	}

	pdata := node.ProvisionPlan
	if pdata.Naming != nil {
		return nil, fnerrors.NewWithLocation(node.Location, "nodes can't provide naming specifications")
	}

	parsed := &ParsedNode{
		Package:         node,
		Startup:         node.ProvisionPlan.Startup,
		ComputePlanWith: append(pdata.ComputePlanWith, combinedProps.ComputePlanWith...),
		ServerFragments: fragments,
	}
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
