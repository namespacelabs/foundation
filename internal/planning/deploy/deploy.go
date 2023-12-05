// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/multiplatform"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/integrations"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/planning/secrets"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/support"
	runtimelibrary "namespacelabs.dev/foundation/library/runtime"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
	"namespacelabs.dev/foundation/std/tasks"
)

var (
	AlsoDeployIngress        = true
	PushPrebuiltsToRegistry  = true
	MirrorPrebuiltToRegistry = false
)

type ResolvedServerImages struct {
	PackageRef     *schema.PackageRef
	Binary         oci.ImageID
	BinaryImage    compute.Computable[oci.ResolvableImage]
	PrebuiltBinary bool
	Config         *oci.ImageID
	Sidecars       []ResolvedBinary
}

type ResolvedBinary struct {
	PackageRef   *schema.PackageRef
	Binary       oci.ImageID
	BinaryConfig *schema.BinaryConfig
}

type serverBuildSpec struct {
	PackageName schema.PackageName
	Binary      compute.Computable[oci.ImageID]
	SourceImage compute.Computable[oci.ResolvableImage]
	Config      compute.Computable[oci.ImageID]
}

type PreparedDeployable struct {
	Ref       *schema.PackageRef
	Template  runtime.DeployableSpec
	SealedCtx pkggraph.SealedContext
	Resources []pkggraph.ResourceInstance
}

func (pd PreparedDeployable) SealedContext() pkggraph.SealedContext {
	return pd.SealedCtx
}

func (pd PreparedDeployable) PackageRef() *schema.PackageRef {
	return pd.Ref
}

func PrepareDeployServers(ctx context.Context, planner planning.Planner, focus ...planning.Server) (compute.Computable[*Plan], error) {
	stack, err := planning.ComputeStack(ctx, focus, planning.ProvisionOpts{Planner: planner.Runtime, PortRange: eval.DefaultPortRange()})
	if err != nil {
		return nil, err
	}

	return PrepareDeployStack(ctx, planner, stack)
}

func PrepareDeployStack(ctx context.Context, planner planning.Planner, stack *planning.Stack, prepared ...compute.Computable[PreparedDeployable]) (compute.Computable[*Plan], error) {
	def, err := prepareHandlerInvocations(ctx, planner, stack)
	if err != nil {
		return nil, err
	}

	ingressResult := computeIngressWithHandlerResult(planner, stack, ingressesFromHandlerResult(def))

	prepare, err := prepareBuildAndDeployment(ctx, planner, stack, def, ingressResult, prepared...)
	if err != nil {
		return nil, err
	}

	fragmentsOnly := compute.Transform("fragments only", ingressResult, func(_ context.Context, r *ComputeIngressResult) ([]*schema.IngressFragment, error) {
		return r.Fragments, nil
	})

	g := &makeDeployGraph{
		stack:            stack,
		prepare:          prepare,
		ingressFragments: fragmentsOnly,
	}

	if AlsoDeployIngress {
		g.ingressPlan = PlanIngressDeployment(planner.Runtime, ingressResult)
	}

	return g, nil
}

func makeBuildAssets(ingressFragments compute.Computable[*ComputeIngressResult]) assets.AvailableBuildAssets {
	return assets.AvailableBuildAssets{
		IngressFragments: compute.Transform("return fragments", ingressFragments, func(_ context.Context, res *ComputeIngressResult) ([]*schema.IngressFragment, error) {
			return res.Fragments, nil
		}),
	}
}

type makeDeployGraph struct {
	stack            *planning.Stack
	prepare          compute.Computable[prepareAndBuildResult]
	ingressFragments compute.Computable[[]*schema.IngressFragment]
	ingressPlan      compute.Computable[*runtime.DeploymentPlan]

	compute.LocalScoped[*Plan]
}

type Plan struct {
	Deployer           *execution.Plan
	ComputedStack      *planning.Stack
	IngressFragments   []*schema.IngressFragment
	Computed           *schema.ComputedConfigurations
	Hints              []string // Optional messages to pass to the user.
	NamespaceReference string
}

func (m *makeDeployGraph) Action() *tasks.ActionEvent {
	return tasks.Action("deploy.make-graph")
}

func (m *makeDeployGraph) Inputs() *compute.In {
	in := compute.Inputs().Computable("prepare", m.prepare).Indigestible("stack", m.stack)
	// TODO predeploy orchestration server already from here?
	if m.ingressFragments != nil {
		in = in.Computable("ingress", m.ingressFragments).Computable("ingressPlan", m.ingressPlan)
	}
	return in
}

func (m *makeDeployGraph) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (m *makeDeployGraph) Compute(ctx context.Context, deps compute.Resolved) (*Plan, error) {
	pbr := compute.MustGetDepValue(deps, m.prepare, "prepare")

	g := execution.NewEmptyPlan()
	g.Add(pbr.HandlerResult.OrderedInvocations...)
	g.Add(pbr.Ops...)

	plan := &Plan{
		Deployer:           g,
		ComputedStack:      m.stack,
		Hints:              pbr.Hints,
		NamespaceReference: pbr.NamespaceReference,
	}

	if ingress, ok := compute.GetDep(deps, m.ingressPlan, "ingressPlan"); ok {
		g.Add(ingress.Value.Definitions...)
	}

	plan.IngressFragments = compute.MustGetDepValue(deps, m.ingressFragments, "ingress")

	// Look for ingress instances in the output of plan resource phase. Any ingresses that were
	// setup via ingress provider will show up here.
	for _, computed := range pbr.ResourcePlan.PlannedResources {
		if computed.InstanceType.ProtoType == "foundation.library.runtime.IngressInstance" {
			instance := &runtimelibrary.IngressInstance{}
			if err := json.Unmarshal(computed.InstanceSerializedJSON, instance); err != nil {
				return nil, err
			}
			plan.IngressFragments = append(plan.IngressFragments, instance.IngressFragment...)
		}
	}

	plan.Computed = pbr.HandlerResult.MergedComputedConfigurations()

	return plan, nil
}

func prepareHandlerInvocations(ctx context.Context, planner planning.Planner, stack *planning.Stack) (compute.Computable[*handlerResult], error) {
	return tasks.Return(ctx, tasks.Action("server.invoke-handlers").
		Arg("env", planner.Context.Environment().Name).
		Scope(stack.AllPackageList().PackageNames()...),
		func(ctx context.Context) (compute.Computable[*handlerResult], error) {
			handlers, err := computeHandlers(ctx, stack)
			if err != nil {
				return nil, err
			}

			// After we've computed the startup plans, issue the necessary provisioning calls.
			return prepareInvokeHandlers(ctx, planner, stack, handlers, protocol.Lifecycle_PROVISION)
		})
}

type prepareAndBuildResult struct {
	HandlerResult      *handlerResult
	ResourcePlan       *resourcePlan
	Ops                []*schema.SerializedInvocation
	Hints              []string
	NamespaceReference string
}

func prepareBuildAndDeployment(ctx context.Context, planner planning.Planner, stack *planning.Stack, stackDef compute.Computable[*handlerResult], ingress compute.Computable[*ComputeIngressResult], prepared ...compute.Computable[PreparedDeployable]) (compute.Computable[prepareAndBuildResult], error) {
	packages, images, err := computeStackAndImages(ctx, planner, stack, serverImagesOpts{
		ProvisionResult:     stackDef,
		IngressFragments:    ingress,
		GenerateConfigImage: true,
	})
	if err != nil {
		return nil, err
	}

	preparedComp := compute.Collect(tasks.Action("deployment.prepared"), prepared...)

	stackWithIngress := compute.Transform("combine-stack-and-ingress", ingress,
		func(_ context.Context, ingress *ComputeIngressResult) (*planning.StackWithIngress, error) {
			return &planning.StackWithIngress{Stack: *stack, IngressFragments: ingress.Fragments}, nil
		})

	resourcePlan := compute.Map(
		tasks.Action("resource.plan-deployment").
			Scope(stack.AllPackageList().PackageNames()...),
		compute.Inputs().
			Indigestible("secrets", planner.Secrets).
			Computable("stackWithIngress", stackWithIngress).
			Computable("prepared", preparedComp),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (*resourcePlan, error) {
			stackWithIngress := compute.MustGetDepValue(deps, stackWithIngress, "stackWithIngress")
			prepared := compute.MustGetDepValue(deps, preparedComp, "prepared")

			var rp resourceList
			for _, ps := range stackWithIngress.Servers {
				if err := rp.checkAddOwnedResources(ctx, stackWithIngress, ps.Server, ps.Resources); err != nil {
					return nil, err
				}
			}

			for _, p := range prepared {
				if err := rp.checkAddOwnedResources(ctx, stackWithIngress, p.Value, p.Value.Resources); err != nil {
					return nil, err
				}
			}

			return planResources(ctx, planner, stackWithIngress, rp)
		})

	imageInputs := compute.Inputs().Indigestible("packages", packages)
	for k, pkg := range packages {
		imageInputs = imageInputs.Computable(pkg.String(), images[k])
	}

	imageIDs := compute.Map(tasks.Action("server.build"),
		imageInputs, compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (map[schema.PackageName]ResolvedServerImages, error) {
			m := map[schema.PackageName]ResolvedServerImages{}
			for k, pkg := range packages {
				srv := compute.MustGetDepValue(deps, images[k], pkg.String())
				m[pkg] = srv
			}
			return m, nil
		})

	deploymentSpec := compute.Map(
		tasks.Action("server.plan-deployment").
			Scope(stack.AllPackageList().PackageNames()...),
		compute.Inputs().
			Indigestible("planner", planner).
			Proto("env", planner.Context.Environment()).
			Computable("resourcePlan", resourcePlan).
			Computable("images", imageIDs).
			Computable("stackAndDefs", stackDef).
			Computable("stackWithIngress", stackWithIngress),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (runtime.DeploymentSpec, error) {
			resourcePlan := compute.MustGetDepValue(deps, resourcePlan, "resourcePlan")
			imageIDs := compute.MustGetDepValue(deps, imageIDs, "images")
			stackAndDefs := compute.MustGetDepValue(deps, stackDef, "stackAndDefs")
			stackWithIngress := compute.MustGetDepValue(deps, stackWithIngress, "stackWithIngress")

			// And finally compute the startup plan of each server in the stack, passing in the id of the
			// images we just built.
			return planDeployment(ctx, planner, stackWithIngress, stackAndDefs.ProvisionOutput, imageIDs, resourcePlan)
		})

	return compute.Map(tasks.Action("plan.combine"), compute.Inputs().
		Indigestible("planner", planner).
		Computable("resourcePlan", resourcePlan).
		Computable("deploymentSpec", deploymentSpec).
		Computable("stackAndDefs", stackDef).
		Computable("prepared", preparedComp), compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (prepareAndBuildResult, error) {
			resourcePlan := compute.MustGetDepValue(deps, resourcePlan, "resourcePlan")
			deploymentSpec := compute.MustGetDepValue(deps, deploymentSpec, "deploymentSpec")
			prepared := compute.MustGetDepValue(deps, preparedComp, "prepared")

			for _, d := range prepared {
				spec := d.Value.Template
				spec.ResourceDeps = resourcePlan.ResourceList.perOwnerResources[d.Value.PackageRef().Canonical()].Dependencies
				spec.PlannedResourceDeps = resourcePlan.ResourceList.perOwnerResources[d.Value.PackageRef().Canonical()].PlannedDependencies
				spec.SecretResources = resourcePlan.ResourceList.perOwnerResources[d.Value.PackageRef().Canonical()].Secrets

				deploymentSpec.Specs = append(deploymentSpec.Specs, spec)
			}

			deploymentPlan, err := planner.Runtime.PlanDeployment(ctx, deploymentSpec)
			if err != nil {
				return prepareAndBuildResult{}, err
			}

			var ops []*schema.SerializedInvocation
			for _, x := range resourcePlan.PlannedResources {
				ops = append(ops, x.Invocations...)
			}
			ops = append(ops, resourcePlan.ExecutionInvocations...)
			ops = append(ops, deploymentPlan.Definitions...)

			return prepareAndBuildResult{
				HandlerResult:      compute.MustGetDepValue(deps, stackDef, "stackAndDefs"),
				ResourcePlan:       resourcePlan,
				Ops:                ops,
				Hints:              deploymentPlan.Hints,
				NamespaceReference: deploymentPlan.NamespaceReference,
			}, nil
		}), nil
}

func planDeployment(ctx context.Context, planner planning.Planner, stack *planning.StackWithIngress, outputs map[schema.PackageName]*provisionOutput, imageIDs map[schema.PackageName]ResolvedServerImages, rp *resourcePlan) (runtime.DeploymentSpec, error) {
	// And finally compute the startup plan of each server in the stack, passing in the id of the
	// images we just built.
	var serverDeployables []runtime.DeployableSpec

	moduleVCS := map[string]*runtimepb.BuildVCS{}
	var errs []error
	for _, srv := range stack.Servers {
		if _, has := moduleVCS[srv.Location.Module.ModuleName()]; has {
			continue
		}

		vcs, err := srv.Location.Module.VCS(ctx)
		if err != nil {
			errs = append(errs, err)
		} else {
			moduleVCS[srv.Location.Module.ModuleName()] = vcs
		}
	}

	if err := multierr.New(errs...); err != nil {
		return runtime.DeploymentSpec{}, err
	}

	for _, srv := range stack.Servers {
		resolved, ok := imageIDs[srv.PackageName()]
		if !ok {
			return runtime.DeploymentSpec{}, fnerrors.InternalError("%s: missing server build results", srv.PackageName())
		}

		var deployable runtime.DeployableSpec

		rt, err := serverToRuntimeConfig(stack, srv, resolved.Binary)
		if err != nil {
			return runtime.DeploymentSpec{}, err
		}

		resources := rp.ResourceList.perOwnerResources

		deployable.MountRuntimeConfigPath = constants.NamespaceConfigMount
		deployable.RuntimeConfig = rt
		deployable.BuildVCS = moduleVCS[srv.Location.Module.ModuleName()]
		deployable.ResourceDeps = resources[srv.PackageRef().Canonical()].Dependencies
		deployable.PlannedResourceDeps = resources[srv.PackageRef().Canonical()].PlannedDependencies
		deployable.SecretResources = resources[srv.PackageRef().Canonical()].Secrets
		deployable.Permissions = srv.MergedFragment.Permissions

		for _, x := range resources[srv.PackageRef().Canonical()].PlannedDependencies {
			found := false
			for _, y := range rp.PlannedResources {
				if x.ResourceInstanceId == y.ResourceInstanceID {
					found = true
					deployable.ComputedResources = append(deployable.ComputedResources, y.ComputedResource)
				}
			}
			if !found {
				return runtime.DeploymentSpec{}, fnerrors.InternalError("referred to resource not found: %s", x.ResourceInstanceId)
			}
		}

		if err := PrepareRunOpts(ctx, &stack.Stack, srv, &resolved, &deployable); err != nil {
			return runtime.DeploymentSpec{}, err
		}

		sidecars := srv.MergedFragment.Sidecar
		inits := srv.MergedFragment.InitContainer

		if err := prepareContainerRunOpts(sidecars, resolved, &deployable.Sidecars); err != nil {
			return runtime.DeploymentSpec{}, err
		}

		if err := prepareContainerRunOpts(inits, resolved, &deployable.Inits); err != nil {
			return runtime.DeploymentSpec{}, err
		}

		if sr := outputs[srv.PackageName()]; sr != nil {
			deployable.Extensions = append(deployable.Extensions, sr.Extensions...)

			for _, ext := range sr.ServerExtensions {
				deployable.Volumes = append(deployable.Volumes, ext.Volume...)

				for _, cext := range ext.ExtendContainer {
					if cext.Name == "" && cext.BinaryRef == nil {
						if err := extendContainer(&deployable.MainContainer, cext); err != nil {
							return runtime.DeploymentSpec{}, err
						}
					} else {
						updatedSidecars, ok, err := checkExtend(deployable.Sidecars, cext)
						if err != nil {
							return runtime.DeploymentSpec{}, err
						}
						if ok {
							deployable.Sidecars = updatedSidecars
						} else {
							updatedInits, ok, err := checkExtend(deployable.Inits, cext)
							if err != nil {
								return runtime.DeploymentSpec{}, err
							}
							if ok {
								deployable.Inits = updatedInits
							} else {
								return runtime.DeploymentSpec{}, fnerrors.BadInputError("%s: no such container", cext.Name)
							}
						}
					}
				}
			}
		}

		for _, ie := range stack.InternalEndpoints {
			if srv.PackageName().Equals(ie.ServerOwner) {
				deployable.InternalEndpoints = append(deployable.InternalEndpoints, ie)
			}
		}

		deployable.Endpoints = stack.Proto().EndpointsBy(srv.PackageName())
		deployable.Focused = stack.Focus.Has(srv.PackageName())

		// Backwards compatibility (e.g. readiness checks for Go application framework use service metadata)
		// TODO refactor.
		probes, err := httpProbes(deployable.Endpoints, deployable.InternalEndpoints)
		if err != nil {
			return runtime.DeploymentSpec{}, err
		}
		deployable.Probes = append(deployable.Probes, probes...)

		var allEnv []*schema.BinaryConfig_EnvEntry
		allEnv = append(allEnv, deployable.MainContainer.Env...)
		for _, container := range deployable.Sidecars {
			allEnv = append(allEnv, container.Env...)
		}
		for _, container := range deployable.Inits {
			allEnv = append(allEnv, container.Env...)
		}

		for _, entry := range allEnv {
			env := entry.Value
			switch {
			case env.FromServiceEndpoint != nil:
				if err := validateServiceRef(env.FromServiceEndpoint, &stack.Stack); err != nil {
					return runtime.DeploymentSpec{}, fnerrors.AttachLocation(srv.Location, err)
				}
			case env.FromServiceIngress != nil:
				if err := validateServiceRef(env.FromServiceIngress, &stack.Stack); err != nil {
					return runtime.DeploymentSpec{}, fnerrors.AttachLocation(srv.Location, err)
				}
			}
		}

		deployable.Secrets = secrets.ScopeSecretsToServer(planner.Secrets, srv.Server)
		serverDeployables = append(serverDeployables, deployable)
	}

	return runtime.DeploymentSpec{
		Specs: serverDeployables,
	}, nil
}

func validateServiceRef(ref *schema.ServiceRef, stack *planning.Stack) error {
	// TODO speed this up.
	for _, e := range stack.Proto().EndpointsBy(ref.ServerRef.AsPackageName()) {
		if e.ServiceName == ref.ServiceName {
			return nil
		}
	}

	// No corresponding endpoint found - check if the server is present in the stack.
	for _, srv := range stack.Servers {
		if srv.Package.PackageName() == ref.ServerRef.AsPackageName() {
			return fnerrors.New("invalid service reference: service %q is not defined for server %q", ref.ServiceName, ref.ServerRef.AsPackageName())
		}
	}

	return fnerrors.UsageError(
		fmt.Sprintf("Try adding %q to you server's `requires` block.", ref.ServerRef.AsPackageName()),
		"invalid service reference: server %q is missing in the deployment stack", ref.ServerRef.AsPackageName())
}

type endpoint interface {
	GetServiceMetadata() []*schema.ServiceMetadata
	GetPorts() []*schema.Endpoint_PortMap
}

func httpProbes(endpoints []*schema.Endpoint, internalEndpoints []*schema.InternalEndpoint) ([]*schema.Probe, error) {
	var probes []*schema.Probe

	for _, e := range endpoints {
		for _, md := range e.GetServiceMetadata() {
			if md.Kind == runtime.FnServiceLivez || md.Kind == runtime.FnServiceReadyz {
				http := &schema.HttpExportedService{}

				if err := md.Details.UnmarshalTo(http); err != nil {
					return nil, fnerrors.InternalError("expected a HttpExportedService: %w", err)
				}

				for _, port := range e.GetPorts() {
					probes = append(probes, &schema.Probe{
						Kind: md.Kind,
						Http: &schema.Probe_Http{
							ContainerPort: port.Port.GetContainerPort(),
							Path:          http.Path,
						},
					})
				}
			}
		}
	}

	for _, ie := range internalEndpoints {
		for _, md := range ie.GetServiceMetadata() {
			if md.Kind == runtime.FnServiceLivez || md.Kind == runtime.FnServiceReadyz {
				http := &schema.HttpExportedService{}

				if err := md.Details.UnmarshalTo(http); err != nil {
					return nil, fnerrors.InternalError("expected a HttpExportedService: %w", err)
				}

				probes = append(probes, &schema.Probe{
					Kind: md.Kind,
					Http: &schema.Probe_Http{
						ContainerPort: ie.GetPort().ContainerPort,
						Path:          http.Path,
					},
				})
			}
		}
	}

	return probes, nil
}

func extendContainer(target *runtime.ContainerRunOpts, cext *schema.ContainerExtension) error {
	target.Mounts = append(target.Mounts, cext.Mount...)
	target.Args = append(target.Args, cext.Args...)
	var err error
	target.Env, err = support.MergeEnvs(target.Env, cext.Env)
	return err
}

func checkExtend(sidecars []runtime.SidecarRunOpts, cext *schema.ContainerExtension) ([]runtime.SidecarRunOpts, bool, error) {
	for k, sidecar := range sidecars {
		if (cext.Name != "" && sidecar.Name == cext.Name) || (cext.BinaryRef != nil && cext.BinaryRef.Equals(sidecar.BinaryRef)) {
			if err := extendContainer(&sidecar.ContainerRunOpts, cext); err != nil {
				return nil, false, err
			}
			sidecars[k] = sidecar
			return sidecars, true, nil
		}
	}

	return nil, false, nil
}

type serverImagesOpts struct {
	IngressFragments    compute.Computable[*ComputeIngressResult]
	ProvisionResult     compute.Computable[*handlerResult]
	GenerateConfigImage bool
}

func (opts serverImagesOpts) computedOnly() compute.Computable[*schema.ComputedConfigurations] {
	return compute.Transform("return computed", opts.ProvisionResult, func(_ context.Context, h *handlerResult) (*schema.ComputedConfigurations, error) {
		return h.MergedComputedConfigurations(), nil
	})
}

func prepareServerImages(ctx context.Context, planner planning.Planner, stack *planning.Stack, opts serverImagesOpts) ([]serverBuildSpec, error) {
	imageList := []serverBuildSpec{}

	for _, srv := range stack.Servers {
		images := serverBuildSpec{PackageName: srv.PackageName()}

		prebuilt, err := binary.PrebuiltImageID(ctx, srv.Location, planner.Context.Configuration())
		if err != nil {
			return nil, err
		}

		var spec build.Spec

		if prebuilt != nil {
			spec = build.PrebuiltPlan(*prebuilt, false /* platformIndependent */, build.PrebuiltResolveOpts())
		} else {
			spec, err = integrations.IntegrationFor(srv.Framework()).PrepareBuild(ctx, makeBuildAssets(opts.IngressFragments), srv, stack.Focus.Has(srv.PackageName()))
		}
		if err != nil {
			return nil, err
		}

		p, err := MakeBuildPlan(ctx, planner.Runtime, srv, stack.Focus.Has(srv.PackageName()), spec)
		if err != nil {
			return nil, err
		}

		poster, err := ensureImage(ctx, srv.Server.SealedContext(), planner.Registry, p)
		if err != nil {
			return nil, err
		}

		images.Binary = poster.ImageID
		images.SourceImage = poster.SourceImage

		// In production builds, also build a "config image" which includes both the processed
		// stack at the time of evaluation of the target image and deployment, but also the
		// source configuration files used to compute a startup configuration, so it can be re-
		// evaluated on a need basis.
		pctx := srv.Server.SealedContext()
		if stack.Focus.Has(srv.PackageName()) && !pctx.Environment().Ephemeral && opts.ProvisionResult != nil && opts.GenerateConfigImage {
			configImage := prepareConfigImage(ctx, planner, srv.Server, stack, opts.computedOnly())
			name := planner.Registry.AllocateName(srv.PackageName().String(), "")
			images.Config = oci.PublishImage(name, configImage).ImageID()
		}

		imageList = append(imageList, images)
	}

	return imageList, nil
}

type imagePoster struct {
	ImageID     compute.Computable[oci.ImageID]
	SourceImage compute.Computable[oci.ResolvableImage]
}

func ensureImage(ctx context.Context, env pkggraph.SealedContext, registry registry.Manager, p build.Plan) (imagePoster, error) {
	if imgid, ok := build.IsPrebuilt(p.Spec); ok {
		if !PushPrebuiltsToRegistry {
			return imagePoster{
				ImageID: build.Prebuilt(imgid),
			}, nil
		}

		if MirrorPrebuiltToRegistry {
			name := registry.AllocateName(p.SourcePackage.String(), "")
			r := oci.Prebuilt(imgid, build.PrebuiltResolveOpts())
			return imagePoster{
				ImageID:     oci.PublishResolvable(name, r, p),
				SourceImage: r,
			}, nil
		}

		// Else, we'll create a minimal prebuilt image with only the platforms required for this deployment.
	}

	name := registry.AllocateName(p.SourcePackage.String(), "")

	// Leave a hint to where we're pushing to, in case the builder can
	// use that information for optimization purposes. This may be
	// replaced with a graph optimization pass in the future.
	p.PublishName = name

	bin, err := multiplatform.PrepareMultiPlatformImage(ctx, env, p)
	if err != nil {
		return imagePoster{}, err
	}

	return imagePoster{
		ImageID:     oci.PublishResolvable(name, bin, p),
		SourceImage: bin,
	}, nil
}

type containerImage struct {
	PackageRef  *schema.PackageRef
	OwnerServer schema.PackageName
	ImageID     compute.Computable[oci.ImageID]
	Command     []string
	Args        []string
	Env         []*schema.BinaryConfig_EnvEntry
	WorkingDir  string
}

func prepareSidecarAndInitImages(ctx context.Context, planner runtime.Planner, registry registry.Manager, stack *planning.Stack, assets assets.AvailableBuildAssets) ([]containerImage, error) {
	res := []containerImage{}
	for k, srv := range stack.Servers {
		platforms, err := planner.TargetPlatforms(ctx)
		if err != nil {
			return nil, err
		}

		sidecars := stack.Servers[k].MergedFragment.Sidecar
		inits := stack.Servers[k].MergedFragment.InitContainer
		sidecars = append(sidecars, inits...) // For our purposes, they are the same.

		for _, container := range sidecars {
			binRef := container.BinaryRef
			if binRef == nil {
				binRef = schema.MakePackageSingleRef(schema.MakePackageName(container.Binary))
			}

			pctx := srv.Server.SealedContext()
			bin, err := pctx.LoadByName(ctx, binRef.AsPackageName())
			if err != nil {
				return nil, err
			}

			prepared, err := binary.Plan(ctx, bin, binRef.Name, pctx, assets,
				binary.BuildImageOpts{
					UsePrebuilts: true,
					Platforms:    platforms,
				})
			if err != nil {
				return nil, err
			}

			poster, err := ensureImage(ctx, pctx, registry, prepared.Plan)
			if err != nil {
				return nil, err
			}

			res = append(res, containerImage{
				PackageRef:  binRef,
				OwnerServer: srv.PackageName(),
				ImageID:     poster.ImageID,
				Command:     prepared.Command,
				Args:        prepared.Args,
				Env:         prepared.Env,
				WorkingDir:  prepared.WorkingDir,
			})
		}
	}
	return res, nil
}

func ComputeStackAndImages(ctx context.Context, planner planning.Planner, servers planning.Servers) (*planning.Stack, []compute.Computable[ResolvedServerImages], error) {
	stack, err := planning.ComputeStack(ctx, servers, planning.ProvisionOpts{Planner: planner.Runtime, PortRange: eval.DefaultPortRange()})
	if err != nil {
		return nil, nil, err
	}

	def, err := prepareHandlerInvocations(ctx, planner, stack)
	if err != nil {
		return nil, nil, err
	}

	ingressFragments := computeIngressWithHandlerResult(planner, stack, ingressesFromHandlerResult(def))

	_, images, err := computeStackAndImages(ctx, planner, stack, serverImagesOpts{
		ProvisionResult:     def,
		IngressFragments:    ingressFragments,
		GenerateConfigImage: false,
	})
	return stack, images, err
}

func computeStackAndImages(ctx context.Context, planner planning.Planner, stack *planning.Stack, opts serverImagesOpts) ([]schema.PackageName, []compute.Computable[ResolvedServerImages], error) {
	imageMap, err := prepareServerImages(ctx, planner, stack, opts)
	if err != nil {
		return nil, nil, err
	}

	sidecarImages, err := prepareSidecarAndInitImages(ctx, planner.Runtime, planner.Registry, stack, makeBuildAssets(opts.IngressFragments))
	if err != nil {
		return nil, nil, err
	}

	var pkgs []schema.PackageName
	var images []compute.Computable[ResolvedServerImages]
	for _, srv := range imageMap {
		srv := srv // Close r.
		in := compute.Inputs().Stringer("package", srv.PackageName).
			Computable("binary", srv.Binary)
		if srv.Config != nil {
			in = in.Computable("config", srv.Config)
		}

		sidecarIndex := 0
		for _, sidecar := range sidecarImages {
			if sidecar.OwnerServer == srv.PackageName {
				in = in.Computable(fmt.Sprintf("sidecar%d", sidecarIndex), sidecar.ImageID)
				sidecarIndex++
			}
		}

		// We make the binary image as indigestible to make it clear that it is
		// also an input below. We just care about retaining the original
		// compute.Computable though.
		in = in.Indigestible("binaryImage", srv.SourceImage)

		pkgs = append(pkgs, srv.PackageName)
		images = append(images, compute.Map(tasks.Action("server.build-images").Scope(srv.PackageName), in, compute.Output{},
			func(ctx context.Context, deps compute.Resolved) (ResolvedServerImages, error) {
				binary, _ := compute.GetDep(deps, srv.Binary, "binary")

				result := ResolvedServerImages{
					PackageRef:     &schema.PackageRef{PackageName: srv.PackageName.String()},
					Binary:         binary.Value,
					BinaryImage:    srv.SourceImage,
					PrebuiltBinary: binary.Completed.IsZero(),
				}

				if v, ok := compute.GetDep(deps, srv.Config, "config"); ok {
					result.Config = &v.Value
				}

				sidecarIndex := 0
				for _, sidecar := range sidecarImages {
					if sidecar.OwnerServer == srv.PackageName {
						if v, ok := compute.GetDep(deps, sidecar.ImageID, fmt.Sprintf("sidecar%d", sidecarIndex)); ok {
							result.Sidecars = append(result.Sidecars, ResolvedBinary{
								PackageRef: sidecar.PackageRef,
								Binary:     v.Value,
								BinaryConfig: &schema.BinaryConfig{
									Command:    sidecar.Command,
									Args:       sidecar.Args,
									Env:        sidecar.Env,
									WorkingDir: sidecar.WorkingDir,
								},
							})
						}
						sidecarIndex++
					}
				}

				return result, nil
			}))
	}

	return pkgs, images, nil
}

func PrepareRunOpts(ctx context.Context, stack *planning.Stack, srv planning.PlannedServer, imgs *ResolvedServerImages, out *runtime.DeployableSpec) error {
	proto := srv.Proto()
	frag := srv.MergedFragment
	main := frag.MainContainer

	out.ErrorLocation = srv.Location
	out.PackageRef = srv.Proto().GetPackageRef()
	out.Class = schema.DeployableClass(proto.DeployableClass)
	out.Replicas = proto.Replicas
	out.Id = proto.Id
	out.Name = proto.Name
	out.Volumes = append(out.Volumes, frag.Volume...)
	out.MainContainer.Mounts = append(out.MainContainer.Mounts, main.Mount...)

	if imgs != nil {
		out.MainContainer.Image = imgs.Binary
		out.ConfigImage = imgs.Config
	}

	out.Probes = frag.Probe
	out.Tolerations = frag.Toleration
	out.Annotations = frag.Annotation
	out.NodeSelector = frag.NodeSelector

	if err := integrations.IntegrationFor(srv.Framework()).PrepareRun(ctx, srv, &out.MainContainer); err != nil {
		return err
	}

	inputs := pkggraph.StartupInputs{
		Stack:         stack.Proto(),
		ServerRootAbs: srv.Location.Abs(),
	}

	if imgs != nil {
		inputs.ServerImage = imgs.Binary.RepoAndDigest()
	}

	if srv.EvalStartup != nil {
		serverStartupPlan, err := srv.EvalStartup(ctx, srv.SealedContext(), inputs, nil)
		if err != nil {
			return err
		}

		out.MainContainer.Args = append(out.MainContainer.Args, serverStartupPlan.Args...)
		out.MainContainer.Env, err = support.MergeEnvs(out.MainContainer.Env, serverStartupPlan.Env)
		if err != nil {
			return err
		}
	}

	stackEntry, ok := stack.Get(srv.PackageName())
	if !ok {
		return fnerrors.InternalError("%s: missing from the stack", srv.PackageName())
	}

	for _, dep := range stackEntry.ParsedDeps {
		plan, err := dep.Startup.EvalStartup(ctx, srv.SealedContext(), inputs, dep.Allocations)
		if err != nil {
			return err
		}

		out.MainContainer.Args = append(out.MainContainer.Args, plan.Args...)
		out.MainContainer.Env, err = support.MergeEnvs(out.MainContainer.Env, plan.Env)
		if err != nil {
			return err
		}
	}

	out.MainContainer.Args = append(out.MainContainer.Args, main.Args...)
	out.MainContainer.Privileged = main.GetSecurity().GetPrivileged()
	out.MainContainer.HostNetwork = main.GetSecurity().GetHostNetwork()
	out.MainContainer.Capabilities = main.GetSecurity().GetCapabilities()
	out.MainContainer.ResourceLimits = main.Limits
	out.MainContainer.ResourceRequests = main.Requests
	out.MainContainer.ContainerPorts = append(out.MainContainer.ContainerPorts, main.ContainerPort...)
	out.MainContainer.TerminationGracePeriodSeconds = main.TerminationGracePeriodSeconds

	var err error
	out.MainContainer.Env, err = support.MergeEnvs(out.MainContainer.Env, main.Env)
	if err != nil {
		return err
	}

	return nil
}

func prepareContainerRunOpts(containers []*schema.Container, resolved ResolvedServerImages, out *[]runtime.SidecarRunOpts) error {
	for _, container := range containers {
		if container.Name == "" {
			return fnerrors.InternalError("%s: sidecar name is required", container.Owner)
		}

		binRef := container.BinaryRef
		if binRef == nil {
			binRef = schema.MakePackageSingleRef(schema.MakePackageName(container.Binary))
		}

		var sidecarBinary *ResolvedBinary
		for _, binary := range resolved.Sidecars {
			if binary.PackageRef.Equals(binRef) {
				sidecarBinary = &binary
				break
			}
		}

		if sidecarBinary == nil {
			return fnerrors.InternalError("%s: missing sidecar build", binRef)
		}

		opts := runtime.SidecarRunOpts{
			Name:      container.Name,
			BinaryRef: binRef,
			ContainerRunOpts: runtime.ContainerRunOpts{
				Image:            sidecarBinary.Binary,
				Args:             append(slices.Clone(sidecarBinary.BinaryConfig.GetArgs()), container.Args...),
				Command:          sidecarBinary.BinaryConfig.GetCommand(),
				WorkingDir:       sidecarBinary.BinaryConfig.GetWorkingDir(),
				Privileged:       container.GetSecurity().GetPrivileged(),
				ResourceLimits:   container.Limits,
				ResourceRequests: container.Requests,
				ContainerPorts:   container.ContainerPort,
				Capabilities:     container.GetSecurity().GetCapabilities(),
			},
		}

		if container.TerminationGracePeriodSeconds > 0 {
			return fnerrors.BadInputError("termination grace period can't be set on sidecars")
		}

		transformed, err := support.MergeEnvs(opts.ContainerRunOpts.Env, sidecarBinary.BinaryConfig.GetEnv())
		if err != nil {
			return err
		}
		opts.ContainerRunOpts.Env, err = support.MergeEnvs(transformed, container.Env)
		if err != nil {
			return err
		}

		*out = append(*out, opts)
	}
	return nil
}
