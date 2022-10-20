// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/multiplatform"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/languages"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/eval"
	"namespacelabs.dev/foundation/internal/planning/startup"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/runtime/constants"
	"namespacelabs.dev/foundation/std/tasks"
)

var (
	AlsoDeployIngress       = true
	PushPrebuiltsToRegistry = false
)

type ResolvedServerImages struct {
	PackageRef     *schema.PackageRef
	Binary         oci.ImageID
	BinaryImage    compute.Computable[oci.ResolvableImage]
	PrebuiltBinary bool
	Config         oci.ImageID
	Sidecars       []ResolvedBinary
}

type ResolvedBinary struct {
	PackageRef *schema.PackageRef
	Binary     oci.ImageID
	Command    []string
}

type serverBuildSpec struct {
	PackageName schema.PackageName
	Binary      compute.Computable[oci.ImageID]
	BinaryImage compute.Computable[oci.ResolvableImage]
	Config      compute.Computable[oci.ImageID]
}

func PrepareDeployServers(ctx context.Context, env cfg.Context, planner runtime.Planner, focus ...planning.Server) (compute.Computable[*Plan], error) {
	reg, err := registry.GetRegistry(ctx, env)
	if err != nil {
		return nil, err
	}

	stack, err := planning.ComputeStack(ctx, focus, planning.ProvisionOpts{PortRange: eval.DefaultPortRange()})
	if err != nil {
		return nil, err
	}

	return PrepareDeployStackToRegistry(ctx, env, planner, reg, stack)
}

func PrepareDeployStack(ctx context.Context, env cfg.Context, planner runtime.Planner, stack *planning.Stack) (compute.Computable[*Plan], error) {
	reg, err := registry.GetRegistry(ctx, env)
	if err != nil {
		return nil, err
	}

	return PrepareDeployStackToRegistry(ctx, env, planner, reg, stack)
}

func PrepareDeployStackToRegistry(ctx context.Context, env cfg.Context, planner runtime.Planner, registry registry.Manager, stack *planning.Stack) (compute.Computable[*Plan], error) {
	def, err := prepareHandlerInvocations(ctx, env, planner, stack)
	if err != nil {
		return nil, err
	}

	ingressFragments := computeIngressWithHandlerResult(env, planner, stack, def)

	prepare, err := prepareBuildAndDeployment(ctx, env, planner, registry, stack, def, makeBuildAssets(ingressFragments))
	if err != nil {
		return nil, err
	}

	g := &makeDeployGraph{
		stack:            stack,
		prepare:          prepare,
		ingressFragments: ingressFragments,
	}

	if AlsoDeployIngress {
		g.ingressPlan = PlanIngressDeployment(planner, g.ingressFragments)
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

func computeIngressWithHandlerResult(env cfg.Context, planner runtime.Planner, stack *planning.Stack, def compute.Computable[*handlerResult]) compute.Computable[*ComputeIngressResult] {
	computedIngressFragments := compute.Transform("parse computed ingress", def, func(ctx context.Context, h *handlerResult) ([]*schema.IngressFragment, error) {
		var fragments []*schema.IngressFragment

		for _, computed := range h.MergedComputedConfigurations().GetEntry() {
			for _, conf := range computed.Configuration {
				p := &schema.IngressFragment{}
				if !conf.Impl.MessageIs(p) {
					continue
				}

				if err := conf.Impl.UnmarshalTo(p); err != nil {
					return nil, err
				}

				fmt.Fprintf(console.Debug(ctx), "%s: received domain: %+v\n", conf.Owner, p.Domain)

				fragments = append(fragments, p)
			}
		}

		return fragments, nil
	})

	return ComputeIngress(env, planner, stack.Proto(), computedIngressFragments, AlsoDeployIngress)
}

type makeDeployGraph struct {
	stack            *planning.Stack
	prepare          compute.Computable[prepareAndBuildResult]
	ingressFragments compute.Computable[*ComputeIngressResult]
	ingressPlan      compute.Computable[*runtime.DeploymentPlan]

	compute.LocalScoped[*Plan]
}

type Plan struct {
	Deployer         *execution.Plan
	ComputedStack    *planning.Stack
	IngressFragments []*schema.IngressFragment
	Computed         *schema.ComputedConfigurations
	Hints            []string // Optional messages to pass to the user.
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
		Deployer:      g,
		ComputedStack: m.stack,
		Hints:         pbr.Hints,
	}

	if ingress, ok := compute.GetDep(deps, m.ingressPlan, "ingressPlan"); ok {
		g.Add(ingress.Value.Definitions...)
	}

	plan.IngressFragments = compute.MustGetDepValue(deps, m.ingressFragments, "ingress").Fragments
	plan.Computed = pbr.HandlerResult.MergedComputedConfigurations()

	return plan, nil
}

func prepareHandlerInvocations(ctx context.Context, env cfg.Context, planner runtime.Planner, stack *planning.Stack) (compute.Computable[*handlerResult], error) {
	return tasks.Return(ctx, tasks.Action("server.invoke-handlers").
		Arg("env", env.Environment().Name).
		Scope(stack.AllPackageList().PackageNames()...),
		func(ctx context.Context) (compute.Computable[*handlerResult], error) {
			handlers, err := computeHandlers(ctx, stack)
			if err != nil {
				return nil, err
			}

			// After we've computed the startup plans, issue the necessary provisioning calls.
			return invokeHandlers(ctx, env, planner, stack, handlers, protocol.Lifecycle_PROVISION)
		})
}

type prepareAndBuildResult struct {
	HandlerResult *handlerResult
	Ops           []*schema.SerializedInvocation
	Hints         []string
}

func prepareBuildAndDeployment(ctx context.Context, env cfg.Context, planner runtime.Planner, registry registry.Manager, stack *planning.Stack, stackDef compute.Computable[*handlerResult], buildAssets assets.AvailableBuildAssets) (compute.Computable[prepareAndBuildResult], error) {
	packages, images, err := computeStackAndImages(ctx, env, planner, registry, stack, stackDef, buildAssets)
	if err != nil {
		return nil, err
	}

	resourcePlan := compute.Map(
		tasks.Action("resource.plan-deployment").
			Scope(stack.AllPackageList().PackageNames()...),
		compute.Inputs().
			Computable("stackAndDefs", stackDef),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (*resourcePlan, error) {
			stackAndDefs := compute.MustGetDepValue(deps, stackDef, "stackAndDefs")

			var rp resourceList
			for _, ps := range stackAndDefs.Stack.Servers {
				if err := rp.checkAddServerResources(ctx, ps.Server, ps.Resources); err != nil {
					return nil, err
				}
			}

			sealedCtx := stack.Servers[0].SealedContext()
			return planResources(ctx, sealedCtx, planner, registry, stackAndDefs.Stack, rp)
		})

	finalInputs := compute.Inputs()

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

	deploymentPlan := compute.Map(
		tasks.Action("server.plan-deployment").
			Scope(stack.AllPackageList().PackageNames()...),
		finalInputs.
			Proto("env", env.Environment()).
			Computable("resourcePlan", resourcePlan).
			Computable("images", imageIDs).
			Computable("stackAndDefs", stackDef),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (*runtime.DeploymentPlan, error) {
			resourcePlan := compute.MustGetDepValue(deps, resourcePlan, "resourcePlan")
			imageIDs := compute.MustGetDepValue(deps, imageIDs, "images")
			stackAndDefs := compute.MustGetDepValue(deps, stackDef, "stackAndDefs")

			// And finally compute the startup plan of each server in the stack, passing in the id of the
			// images we just built.
			return planDeployment(ctx, env.Environment(), planner, stackAndDefs.Stack, stackAndDefs.ProvisionOutput, imageIDs, resourcePlan.ResourceList.perServerResources)
		})

	return compute.Map(tasks.Action("plan.combine"), compute.Inputs().
		Computable("resourcePlan", resourcePlan).
		Computable("deploymentPlan", deploymentPlan).
		Computable("stackAndDefs", stackDef), compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (prepareAndBuildResult, error) {
			resourcePlan := compute.MustGetDepValue(deps, resourcePlan, "resourcePlan")
			deploymentPlan := compute.MustGetDepValue(deps, deploymentPlan, "deploymentPlan")
			return prepareAndBuildResult{
				HandlerResult: compute.MustGetDepValue(deps, stackDef, "stackAndDefs"),
				Ops:           append(resourcePlan.Invocations, deploymentPlan.Definitions...),
				Hints:         deploymentPlan.Hints,
			}, nil
		}), nil
}

func planDeployment(ctx context.Context, env *schema.Environment, planner runtime.Planner, stack *planning.Stack, outputs map[schema.PackageName]*provisionOutput, imageIDs map[schema.PackageName]ResolvedServerImages, resources map[schema.PackageName]serverResourceInstances) (*runtime.DeploymentPlan, error) {
	// And finally compute the startup plan of each server in the stack, passing in the id of the
	// images we just built.
	var serverRuns []runtime.DeployableSpec
	var secretSources []secretSource

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
		return nil, err
	}

	for k, srv := range stack.Servers {
		resolved, ok := imageIDs[srv.PackageName()]
		if !ok {
			return nil, fnerrors.InternalError("%s: missing server build results", srv.PackageName())
		}

		var run runtime.DeployableSpec

		rt, err := serverToRuntimeConfig(stack, srv, resolved.Binary)
		if err != nil {
			return nil, err
		}

		run.MountRuntimeConfigPath = constants.NamespaceConfigMount
		run.RuntimeConfig = rt
		run.BuildVCS = moduleVCS[srv.Location.Module.ModuleName()]
		run.Resources = resources[srv.PackageName()].Dependencies
		run.SecretResources = resources[srv.PackageName()].Secrets

		if err := prepareRunOpts(ctx, stack, srv.Server, resolved, &run); err != nil {
			return nil, err
		}

		sidecars, inits := stack.Servers[k].SidecarsAndInits()

		if err := prepareContainerRunOpts(sidecars, resolved, &run.Sidecars); err != nil {
			return nil, err
		}

		if err := prepareContainerRunOpts(inits, resolved, &run.Inits); err != nil {
			return nil, err
		}

		if sr := outputs[srv.PackageName()]; sr != nil {
			run.Extensions = append(run.Extensions, sr.Extensions...)

			for _, ext := range sr.ServerExtensions {
				run.Volumes = append(run.Volumes, ext.Volume...)

				for _, cext := range ext.ExtendContainer {
					if cext.Name == "" && cext.BinaryRef == nil {
						extendContainer(&run.MainContainer, cext)
					} else {
						if updatedSidecars, ok := checkExtend(run.Sidecars, cext); ok {
							run.Sidecars = updatedSidecars
						} else if updatedInits, ok := checkExtend(run.Inits, cext); ok {
							run.Inits = updatedInits
						} else {
							return nil, fnerrors.BadInputError("%s: no such container", cext.Name)
						}
					}
				}
			}
		}

		for _, ie := range stack.Proto().InternalEndpoint {
			if srv.PackageName().Equals(ie.ServerOwner) {
				run.InternalEndpoints = append(run.InternalEndpoints, ie)
			}
		}

		run.Endpoints = stack.Proto().EndpointsBy(srv.PackageName())
		run.Focused = stack.Focus.Includes(srv.PackageName())

		// Collect all secret references.
		var secretRefs []*schema.PackageRef
		// TODO collect secret refs from resources, too.
		for _, v := range run.Volumes {
			if v.Kind == constants.VolumeKindConfigurable {
				cv := &schema.ConfigurableVolume{}
				if err := v.Definition.UnmarshalTo(cv); err != nil {
					return nil, fnerrors.InternalError("%s: failed to unmarshal configurable volume definition: %w", v.Name, err)
				}

				for _, e := range cv.Entries {
					if e.SecretRef != nil && e.SecretRef.Name != "" {
						secretRefs = append(secretRefs, e.SecretRef)
					}
				}
			}
		}

		var allEnv []*schema.BinaryConfig_EnvEntry
		allEnv = append(allEnv, run.MainContainer.Env...)
		for _, container := range run.Sidecars {
			allEnv = append(allEnv, container.Env...)
		}
		for _, container := range run.Inits {
			allEnv = append(allEnv, container.Env...)
		}

		for _, env := range allEnv {
			if env.FromSecretRef != nil {
				secretRefs = append(secretRefs, env.FromSecretRef)
			}
		}

		serverRuns = append(serverRuns, run)
		secretSources = append(secretSources, secretSource{srv.Server, secretRefs})
	}

	grounded, err := loadSecrets(ctx, env, secretSources...)
	if err != nil {
		return nil, err
	}

	return planner.PlanDeployment(ctx, runtime.DeploymentSpec{
		Specs:   serverRuns,
		Secrets: *grounded,
	})
}

func extendContainer(target *runtime.ContainerRunOpts, cext *schema.ContainerExtension) {
	target.Mounts = append(target.Mounts, cext.Mount...)
	target.Args = append(target.Args, cext.Args...)
	target.Env = append(target.Env, cext.Env...)
}

func checkExtend(sidecars []runtime.SidecarRunOpts, cext *schema.ContainerExtension) ([]runtime.SidecarRunOpts, bool) {
	for k, sidecar := range sidecars {
		if (cext.Name != "" && sidecar.Name == cext.Name) || (cext.BinaryRef != nil && cext.BinaryRef.Equals(sidecar.BinaryRef)) {
			extendContainer(&sidecar.ContainerRunOpts, cext)
			sidecars[k] = sidecar
			return sidecars, true
		}
	}

	return nil, false
}

func prepareServerImages(ctx context.Context, env cfg.Context, planner runtime.Planner,
	registry registry.Manager, stack *planning.Stack, buildAssets assets.AvailableBuildAssets,
	computedConfigs compute.Computable[*schema.ComputedConfigurations]) ([]serverBuildSpec, error) {
	imageList := []serverBuildSpec{}

	for _, srv := range stack.Servers {
		images := serverBuildSpec{PackageName: srv.PackageName()}

		prebuilt, err := binary.PrebuiltImageID(ctx, srv.Location, env.Configuration())
		if err != nil {
			return nil, err
		}

		var spec build.Spec

		if prebuilt != nil {
			spec = build.PrebuiltPlan(*prebuilt, false /* platformIndependent */, build.PrebuiltResolveOpts())
		} else {
			spec, err = languages.IntegrationFor(srv.Framework()).PrepareBuild(ctx, buildAssets, srv.Server, stack.Focus.Includes(srv.PackageName()))
		}
		if err != nil {
			return nil, err
		}

		p, err := MakePlan(ctx, planner, srv.Server, spec)
		if err != nil {
			return nil, err
		}

		images.Binary, images.BinaryImage, err = ensureImage(ctx, srv.Server.SealedContext(), registry, p)
		if err != nil {
			return nil, err
		}

		// In production builds, also build a "config image" which includes both the processed
		// stack at the time of evaluation of the target image and deployment, but also the
		// source configuration files used to compute a startup configuration, so it can be re-
		// evaluated on a need basis.
		pctx := srv.Server.SealedContext()
		if stack.Focus.Includes(srv.PackageName()) && !pctx.Environment().Ephemeral && computedConfigs != nil {
			configImage := prepareConfigImage(ctx, env, planner, srv.Server, stack, computedConfigs)
			name := registry.AllocateName(srv.PackageName().String())
			images.Config = oci.PublishImage(name, configImage).ImageID()
		}

		imageList = append(imageList, images)
	}

	return imageList, nil
}

func ensureImage(ctx context.Context, env pkggraph.SealedContext, registry registry.Manager, p build.Plan) (compute.Computable[oci.ImageID], compute.Computable[oci.ResolvableImage], error) {
	if imgid, ok := build.IsPrebuilt(p.Spec); ok && !PushPrebuiltsToRegistry {
		return build.Prebuilt(imgid), nil, nil
	}

	name := registry.AllocateName(p.SourcePackage.String())

	// Leave a hint to where we're pushing to, in case the builder can
	// use that information for optimization purposes. This may be
	// replaced with a graph optimization pass in the future.
	p.PublishName = name

	bin, err := multiplatform.PrepareMultiPlatformImage(ctx, env, p)
	if err != nil {
		return nil, nil, err
	}

	return oci.PublishResolvable(name, bin), bin, nil
}

type containerImage struct {
	PackageRef  *schema.PackageRef
	OwnerServer schema.PackageName
	Image       compute.Computable[oci.ImageID]
	Command     []string
}

func prepareSidecarAndInitImages(ctx context.Context, planner runtime.Planner, registry registry.Manager, stack *planning.Stack, assets assets.AvailableBuildAssets) ([]containerImage, error) {
	res := []containerImage{}
	for k, srv := range stack.Servers {
		platforms, err := planner.TargetPlatforms(ctx)
		if err != nil {
			return nil, err
		}

		sidecars, inits := stack.Servers[k].SidecarsAndInits()
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

			pbin, _, err := ensureImage(ctx, pctx, registry, prepared.Plan)
			if err != nil {
				return nil, err
			}

			res = append(res, containerImage{
				PackageRef:  binRef,
				OwnerServer: srv.PackageName(),
				Image:       pbin,
				Command:     prepared.Command,
			})
		}
	}
	return res, nil
}

func ComputeStackAndImages(ctx context.Context, env cfg.Context, planner runtime.Planner, servers planning.Servers) (*planning.Stack, []compute.Computable[ResolvedServerImages], error) {
	reg, err := registry.GetRegistry(ctx, env)
	if err != nil {
		return nil, nil, err
	}

	stack, err := planning.ComputeStack(ctx, servers, planning.ProvisionOpts{PortRange: eval.DefaultPortRange()})
	if err != nil {
		return nil, nil, err
	}

	def, err := prepareHandlerInvocations(ctx, env, planner, stack)
	if err != nil {
		return nil, nil, err
	}

	ingressFragments := computeIngressWithHandlerResult(env, planner, stack, def)

	_, images, err := computeStackAndImages(ctx, env, planner, reg, stack, def, makeBuildAssets(ingressFragments))
	return stack, images, err
}

func computeStackAndImages(ctx context.Context, env cfg.Context, planner runtime.Planner, registry registry.Manager, stack *planning.Stack, def compute.Computable[*handlerResult], buildAssets assets.AvailableBuildAssets) ([]schema.PackageName, []compute.Computable[ResolvedServerImages], error) {
	computedOnly := compute.Transform("return computed", def, func(_ context.Context, h *handlerResult) (*schema.ComputedConfigurations, error) {
		return h.MergedComputedConfigurations(), nil
	})

	imageMap, err := prepareServerImages(ctx, env, planner, registry, stack, buildAssets, computedOnly)
	if err != nil {
		return nil, nil, err
	}

	sidecarImages, err := prepareSidecarAndInitImages(ctx, planner, registry, stack, buildAssets)
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
				in = in.Computable(fmt.Sprintf("sidecar%d", sidecarIndex), sidecar.Image)
				sidecarIndex++
			}
		}

		// We make the binary image as indigestible to make it clear that it is
		// also an input below. We just care about retaining the original
		// compute.Computable though.
		in = in.Indigestible("binaryImage", srv.BinaryImage)

		pkgs = append(pkgs, srv.PackageName)
		images = append(images, compute.Map(tasks.Action("server.build-images").Scope(srv.PackageName), in, compute.Output{},
			func(ctx context.Context, deps compute.Resolved) (ResolvedServerImages, error) {
				binary, _ := compute.GetDep(deps, srv.Binary, "binary")

				result := ResolvedServerImages{
					PackageRef:     &schema.PackageRef{PackageName: srv.PackageName.String()},
					Binary:         binary.Value,
					BinaryImage:    srv.BinaryImage,
					PrebuiltBinary: binary.Completed.IsZero(),
				}

				if v, ok := compute.GetDep(deps, srv.Config, "config"); ok {
					result.Config = v.Value
				}

				sidecarIndex := 0
				for _, sidecar := range sidecarImages {
					if sidecar.OwnerServer == srv.PackageName {
						if v, ok := compute.GetDep(deps, sidecar.Image, fmt.Sprintf("sidecar%d", sidecarIndex)); ok {
							result.Sidecars = append(result.Sidecars, ResolvedBinary{
								PackageRef: sidecar.PackageRef,
								Binary:     v.Value,
								Command:    sidecar.Command,
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

func prepareRunOpts(ctx context.Context, stack *planning.Stack, srv planning.Server, imgs ResolvedServerImages, out *runtime.DeployableSpec) error {
	proto := srv.Proto()
	out.ErrorLocation = srv.Location
	out.PackageName = srv.PackageName()
	out.Class = schema.DeployableClass(proto.DeployableClass)
	out.Id = proto.Id
	out.Name = proto.Name
	out.Volumes = append(out.Volumes, proto.Volume...)
	out.MainContainer.Mounts = append(out.MainContainer.Mounts, proto.MainContainer.Mount...)

	out.MainContainer.Image = imgs.Binary
	if imgs.Config.Repository != "" {
		out.ConfigImage = &imgs.Config
	}

	if err := languages.IntegrationFor(srv.Framework()).PrepareRun(ctx, srv, &out.MainContainer); err != nil {
		return err
	}

	inputs := pkggraph.StartupInputs{
		Stack:         stack.Proto(),
		ServerImage:   imgs.Binary.RepoAndDigest(),
		ServerRootAbs: srv.Location.Abs(),
	}

	serverStartupPlan, err := srv.Startup.EvalStartup(ctx, srv.SealedContext(), inputs, nil)
	if err != nil {
		return err
	}

	stackEntry, ok := stack.Get(srv.PackageName())
	if !ok {
		return fnerrors.InternalError("%s: missing from the stack", srv.PackageName())
	}

	merged, err := startup.ComputeConfig(ctx, srv.SealedContext(), serverStartupPlan, stackEntry.ParsedDeps, inputs)
	if err != nil {
		return err
	}

	if merged.WorkingDir != "" {
		out.MainContainer.WorkingDir = merged.WorkingDir
	}
	out.MainContainer.Args = append(out.MainContainer.Args, merged.Args...)
	out.MainContainer.Env = append(out.MainContainer.Env, srv.Proto().MainContainer.Env...)
	out.MainContainer.Env = append(out.MainContainer.Env, merged.Env...)

	return nil
}

func prepareContainerRunOpts(containers []*schema.SidecarContainer, resolved ResolvedServerImages, out *[]runtime.SidecarRunOpts) error {
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

		*out = append(*out, runtime.SidecarRunOpts{
			Name:      container.Name,
			BinaryRef: binRef,
			ContainerRunOpts: runtime.ContainerRunOpts{
				Image:   sidecarBinary.Binary,
				Args:    container.Args,
				Env:     container.Env,
				Command: sidecarBinary.Command,
			},
		})
	}
	return nil
}
