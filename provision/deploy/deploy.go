// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/integrations/shared"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/startup"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	AlsoDeployIngress       = true
	PushPrebuiltsToRegistry = false
)

type serverImages struct {
	PackageRef *schema.PackageRef
	Binary     compute.Computable[oci.ImageID]
	Config     compute.Computable[oci.ImageID]
}

type ResolvedServerImages struct {
	PackageRef     *schema.PackageRef
	Binary         oci.ImageID
	PrebuiltBinary bool
	Config         oci.ImageID
	Sidecars       []ResolvedSidecarImage
}

type ResolvedSidecarImage struct {
	PackageRef *schema.PackageRef
	Binary     oci.ImageID
}

func PrepareDeployServers(ctx context.Context, env planning.Context, focus []provision.Server, onStack func(*stack.Stack)) (compute.Computable[*Plan], error) {
	stack, err := stack.Compute(ctx, focus, stack.ProvisionOpts{PortRange: runtime.DefaultPortRange()})
	if err != nil {
		return nil, err
	}

	if onStack != nil {
		onStack(stack)
	}

	return PrepareDeployStack(ctx, env, stack, focus)
}

func PrepareDeployStack(ctx context.Context, env planning.Context, stack *stack.Stack, focus []provision.Server) (compute.Computable[*Plan], error) {
	for _, srv := range stack.Servers {
		if err := provision.CheckCompatible(srv); err != nil {
			return nil, err
		}
	}

	def, err := prepareHandlerInvocations(ctx, env, stack)
	if err != nil {
		return nil, err
	}

	ingressFragments := computeIngressWithHandlerResult(env, stack, def)

	prepare, err := prepareBuildAndDeployment(ctx, env, focus, stack, def, makeBuildAssets(ingressFragments))
	if err != nil {
		return nil, err
	}

	g := &makeDeployGraph{
		stack:            stack,
		prepare:          prepare,
		ingressFragments: ingressFragments,
	}

	if AlsoDeployIngress {
		g.ingressPlan = PlanIngressDeployment(g.ingressFragments)
	}

	return g, nil
}

func makeBuildAssets(ingressFragments compute.Computable[*ComputeIngressResult]) languages.AvailableBuildAssets {
	return languages.AvailableBuildAssets{
		IngressFragments: compute.Transform(ingressFragments, func(_ context.Context, res *ComputeIngressResult) ([]*schema.IngressFragment, error) {
			return res.Fragments, nil
		}),
	}
}

func computeIngressWithHandlerResult(env planning.Context, stack *stack.Stack, def compute.Computable[*handlerResult]) compute.Computable[*ComputeIngressResult] {
	computedIngressFragments := compute.Transform(def, func(ctx context.Context, h *handlerResult) ([]*schema.IngressFragment, error) {
		var fragments []*schema.IngressFragment

		for _, computed := range h.Computed.GetEntry() {
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

	return ComputeIngress(env, stack.Proto(), computedIngressFragments, AlsoDeployIngress)
}

type makeDeployGraph struct {
	stack            *stack.Stack
	prepare          compute.Computable[prepareAndBuildResult]
	ingressFragments compute.Computable[*ComputeIngressResult]
	ingressPlan      compute.Computable[runtime.DeploymentState]

	compute.LocalScoped[*Plan]
}

type Plan struct {
	Deployer         *ops.Plan
	ComputedStack    *stack.Stack
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

	g := ops.NewPlan()

	if err := g.Add(pbr.HandlerResult.Definitions...); err != nil {
		return nil, err
	}

	if err := g.Add(pbr.DeploymentState.Definitions()...); err != nil {
		return nil, err
	}

	plan := &Plan{
		Deployer:      g,
		ComputedStack: m.stack,
		Hints:         pbr.DeploymentState.Hints(),
	}

	if ingress, ok := compute.GetDep(deps, m.ingressPlan, "ingressPlan"); ok {
		if err := g.Add(ingress.Value.Definitions()...); err != nil {
			return nil, err
		}
	}

	plan.IngressFragments = compute.MustGetDepValue(deps, m.ingressFragments, "ingress").Fragments
	plan.Computed = pbr.HandlerResult.Computed

	return plan, nil
}

func prepareHandlerInvocations(ctx context.Context, env planning.Context, stack *stack.Stack) (compute.Computable[*handlerResult], error) {
	return tasks.Return(ctx, tasks.Action("server.invoke-handlers").
		Arg("env", env.Environment().Name).
		Scope(provision.ServerPackages(stack.Servers).PackageNames()...),
		func(ctx context.Context) (compute.Computable[*handlerResult], error) {
			handlers, err := computeHandlers(ctx, stack)
			if err != nil {
				return nil, err
			}

			// After we've computed the startup plans, issue the necessary provisioning calls.
			return invokeHandlers(ctx, env, stack, handlers, protocol.Lifecycle_PROVISION)
		})
}

type prepareAndBuildResult struct {
	HandlerResult   *handlerResult
	DeploymentState runtime.DeploymentState
}

type sidecarPackage struct {
	PackageRef *schema.PackageRef
	Command    []string
}

type builtImage struct {
	PackageRef *schema.PackageRef
	Binary     oci.ImageID
	Config     oci.ImageID
}

type builtImages []builtImage

func (bi builtImages) get(ref *schema.PackageRef) (builtImage, bool) {
	for _, p := range bi {
		if p.PackageRef.Equals(ref) {
			return p, true
		}
	}
	return builtImage{}, false
}

func prepareBuildAndDeployment(ctx context.Context, env planning.Context, servers []provision.Server, stack *stack.Stack, stackDef compute.Computable[*handlerResult], buildAssets languages.AvailableBuildAssets) (compute.Computable[prepareAndBuildResult], error) {
	var focus schema.PackageList
	for _, server := range servers {
		focus.Add(server.PackageName())
	}

	computedOnly := compute.Transform(stackDef, func(_ context.Context, h *handlerResult) (*schema.ComputedConfigurations, error) {
		return h.Computed, nil
	})

	// computedOnly is used exclusively by config images. They include the set of
	// computed configurations that provision tools may have emitted.
	imgs, err := prepareServerImages(ctx, env, focus, stack, buildAssets, computedOnly)
	if err != nil {
		return nil, err
	}

	sidecarImages, err := prepareSidecarAndInitImages(ctx, stack)
	if err != nil {
		return nil, err
	}

	finalInputs := compute.Inputs()

	var sidecarCommands []sidecarPackage
	for _, v := range sidecarImages {
		// There's an assumption here that sidecar/init packages are non-overlapping with servers.
		imgs = append(imgs, serverImages{
			PackageRef: v.PackageRef,
			Binary:     v.Image,
		})
		sidecarCommands = append(sidecarCommands, sidecarPackage{PackageRef: v.PackageRef, Command: v.Command})
	}

	// Stable ordering.
	sort.Slice(sidecarCommands, func(i, j int) bool {
		return strings.Compare(sidecarCommands[i].PackageRef.String(), sidecarCommands[j].PackageRef.String()) < 0
	})

	// Ensure sidecarCommands are part of the cache key.
	finalInputs = finalInputs.JSON("sidecarCommands", sidecarCommands)

	// A two-layer graph is created here: the first layer depends on all the server binaries,
	// while the second layer depends on all config images (if specified), plus depending on
	// the outcome of invoking all handlers, and then the outcome of all server images. This
	// allows all builds and invocations to occur in parallel.

	binaryInputs := compute.Inputs()

	sort.Slice(imgs, func(i, j int) bool {
		return imgs[i].PackageRef.Compare(imgs[j].PackageRef) < 0
	})
	for _, img := range imgs {
		if img.Binary != nil {
			binaryInputs = binaryInputs.Computable(fmt.Sprintf("%s:binary", img.PackageRef.CanonicalString()), img.Binary)
		}
		if img.Config != nil {
			binaryInputs = binaryInputs.Computable(fmt.Sprintf("%s:config", img.PackageRef.CanonicalString()), img.Config)
		}
	}

	imageIDs := compute.Map(tasks.Action(runtime.TaskServerBuild),
		binaryInputs, compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (builtImages, error) {
			var built builtImages

			for _, img := range imgs {
				imgResult, ok := compute.GetDepWithType[oci.ImageID](deps, fmt.Sprintf("%s:binary", img.PackageRef.CanonicalString()))
				if !ok {
					return nil, fnerrors.InternalError("server image missing")
				}

				b := builtImage{
					PackageRef: img.PackageRef,
					Binary:     imgResult.Value,
				}

				if v, ok := compute.GetDepWithType[oci.ImageID](deps, fmt.Sprintf("%s:config", img.PackageRef.CanonicalString())); ok {
					b.Config = v.Value
				}

				built = append(built, b)
			}

			slices.SortFunc(built, func(a, b builtImage) bool {
				return strings.Compare(a.PackageRef.String(), b.PackageRef.String()) < 0
			})

			return built, nil
		})

	secretData := compute.Map(
		tasks.Action("server.collect-secret-data").
			Scope(provision.ServerPackages(stack.Servers).PackageNames()...),
		compute.Inputs().
			Proto("env", env.Environment()).
			Computable("stackAndDefs", stackDef),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, deps compute.Resolved) (*runtime.GroundedSecrets, error) {
			handlerR := compute.MustGetDepValue(deps, stackDef, "stackAndDefs")

			return loadSecrets(ctx, env.Environment(), handlerR.Stack)
		})

	c1 := compute.Map(
		tasks.Action(runtime.TaskServerProvision).
			Scope(provision.ServerPackages(stack.Servers).PackageNames()...),
		finalInputs.
			Computable("images", imageIDs).
			Computable("stackAndDefs", stackDef).
			Computable("secretData", secretData),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (prepareAndBuildResult, error) {
			imageIDs := compute.MustGetDepValue(deps, imageIDs, "images")
			handlerR := compute.MustGetDepValue(deps, stackDef, "stackAndDefs")
			secrets := compute.MustGetDepValue(deps, secretData, "secretData")

			stack := handlerR.Stack

			// And finally compute the startup plan of each server in the stack, passing in the id of the
			// images we just built.
			var serverRuns []runtime.ServerConfig
			for k, srv := range stack.Servers {
				img, ok := imageIDs.get(srv.PackageRef())
				if !ok {
					return prepareAndBuildResult{}, fnerrors.InternalError("%s: missing an image to run", srv.PackageName())
				}

				var run runtime.ServerConfig

				run.RuntimeConfig, err = serverToRuntimeConfig(stack, srv, img.Binary)
				if err != nil {
					return prepareAndBuildResult{}, err
				}

				if err := prepareRunOpts(ctx, stack, srv, img, &run); err != nil {
					return prepareAndBuildResult{}, err
				}

				sidecars, inits := stack.ParsedServers[k].SidecarsAndInits()

				if err := prepareContainerRunOpts(sidecars, imageIDs, sidecarCommands, &run.Sidecars); err != nil {
					return prepareAndBuildResult{}, err
				}

				if err := prepareContainerRunOpts(inits, imageIDs, sidecarCommands, &run.Inits); err != nil {
					return prepareAndBuildResult{}, err
				}

				if sr := handlerR.ServerDefs[srv.PackageName()]; sr != nil {
					run.Extensions = sr.Extensions
					run.ServerExtensions = sr.ServerExtensions
				}

				serverRuns = append(serverRuns, run)
			}

			deployment, err := runtime.For(ctx, env).PlanDeployment(ctx, runtime.Deployment{
				Focus:   focus,
				Stack:   stack.Proto(),
				Servers: serverRuns,
				Secrets: *secrets,
			})
			if err != nil {
				return prepareAndBuildResult{}, err
			}

			return prepareAndBuildResult{
				HandlerResult:   handlerR,
				DeploymentState: deployment,
			}, nil
		})

	return c1, nil
}

func loadWorkspaceSecrets(ctx context.Context, keyDir fs.FS, module *pkggraph.Module) (*secrets.Bundle, error) {
	contents, err := fs.ReadFile(module.ReadOnlyFS(), secrets.WorkspaceBundleName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fnerrors.InternalError("%s: failed to read %q: %w", module.Workspace.ModuleName, secrets.ServerBundleName, err)
	}

	return secrets.LoadBundle(ctx, keyDir, contents)
}

func prepareServerImages(ctx context.Context, env planning.Context,
	focus schema.PackageList, stack *stack.Stack,
	buildAssets languages.AvailableBuildAssets,
	computedConfigs compute.Computable[*schema.ComputedConfigurations]) ([]serverImages, error) {
	imageList := []serverImages{}

	for _, srv := range stack.Servers {
		images := serverImages{PackageRef: srv.PackageRef()}

		var err error
		var spec build.Spec
		if srv.Integration() != nil {
			spec, err = shared.PrepareBuild(ctx, srv.Location, srv.Integration(), focus.Includes(srv.PackageName()))
		} else {
			spec, err = languages.IntegrationFor(srv.Framework()).PrepareBuild(ctx, buildAssets, srv, focus.Includes(srv.PackageName()))
		}
		if err != nil {
			return nil, err
		}

		if imgid, ok := build.IsPrebuilt(spec); ok && !PushPrebuiltsToRegistry {
			images.Binary = build.Prebuilt(imgid)
		} else {
			p, err := makePlan(ctx, srv, spec)
			if err != nil {
				return nil, err
			}

			name, err := registry.AllocateName(ctx, srv.SealedContext(), srv.PackageName())
			if err != nil {
				return nil, err
			}

			// Leave a hint to where we're pushing to, in case the builder can
			// use that information for optimization purposes. This may be
			// replaced with a graph optimization pass in the future.
			p.PublishName = name

			bin, err := multiplatform.PrepareMultiPlatformImage(ctx, srv.SealedContext(), p)
			if err != nil {
				return nil, err
			}

			images.Binary = oci.PublishResolvable(name, bin)
		}

		// In production builds, also build a "config image" which includes both the processed
		// stack at the time of evaluation of the target image and deployment, but also the
		// source configuration files used to compute a startup configuration, so it can be re-
		// evaluated on a need basis.
		if focus.Includes(srv.PackageName()) && !srv.SealedContext().Environment().Ephemeral && computedConfigs != nil {
			configImage := prepareConfigImage(ctx, env, srv, stack, computedConfigs)

			cfgtag, err := registry.AllocateName(ctx, srv.SealedContext(), srv.PackageName())
			if err != nil {
				return nil, err
			}

			images.Config = oci.PublishImage(cfgtag, configImage).ImageID()
		}

		imageList = append(imageList, images)
	}

	return imageList, nil
}

type containerImage struct {
	PackageRef  *schema.PackageRef
	OwnerServer *schema.PackageRef
	Image       compute.Computable[oci.ImageID]
	Command     []string
}

func prepareSidecarAndInitImages(ctx context.Context, stack *stack.Stack) ([]containerImage, error) {
	res := []containerImage{}
	for k, srv := range stack.Servers {
		platforms, err := runtime.TargetPlatforms(ctx, srv.SealedContext())
		if err != nil {
			return nil, err
		}

		sidecars, inits := stack.ParsedServers[k].SidecarsAndInits()
		sidecars = append(sidecars, inits...) // For our purposes, they are the same.

		for _, container := range sidecars {
			binRef := container.BinaryRef
			if binRef == nil {
				binRef = schema.NewPackageRef(schema.Name(container.Binary), "")
			}

			pkgname := binRef.PackageName()

			bin, err := srv.SealedContext().LoadByName(ctx, pkgname)
			if err != nil {
				return nil, err
			}

			prepared, err := binary.Plan(ctx, bin, binRef.Name,
				binary.BuildImageOpts{
					UsePrebuilts: true,
					Platforms:    platforms,
				})
			if err != nil {
				return nil, err
			}

			image, err := prepared.Image(ctx, srv.SealedContext())
			if err != nil {
				return nil, err
			}

			tag, err := registry.AllocateName(ctx, srv.SealedContext(), bin.PackageName())
			if err != nil {
				return nil, err
			}

			res = append(res, containerImage{
				PackageRef:  binRef,
				OwnerServer: srv.PackageRef(),
				Image:       oci.PublishResolvable(tag, image),
				Command:     prepared.Command,
			})
		}
	}
	return res, nil
}

func ComputeStackAndImages(ctx context.Context, env planning.Context, servers []provision.Server) (*stack.Stack, []compute.Computable[ResolvedServerImages], error) {
	stack, err := stack.Compute(ctx, servers, stack.ProvisionOpts{PortRange: runtime.DefaultPortRange()})
	if err != nil {
		return nil, nil, err
	}

	def, err := prepareHandlerInvocations(ctx, env, stack)
	if err != nil {
		return nil, nil, err
	}

	ingressFragments := computeIngressWithHandlerResult(env, stack, def)

	computedOnly := compute.Transform(def, func(_ context.Context, h *handlerResult) (*schema.ComputedConfigurations, error) {
		return h.Computed, nil
	})

	imageMap, err := prepareServerImages(ctx, env, provision.ServerPackages(servers), stack, makeBuildAssets(ingressFragments), computedOnly)
	if err != nil {
		return nil, nil, err
	}

	sidecarImages, err := prepareSidecarAndInitImages(ctx, stack)
	if err != nil {
		return nil, nil, err
	}

	var images []compute.Computable[ResolvedServerImages]
	for _, r := range imageMap {
		r := r // Close r.
		in := compute.Inputs().Stringer("package", r.PackageRef).Computable("binary", r.Binary)
		if r.Config != nil {
			in = in.Computable("config", r.Config)
		}

		sidecarIndex := 0
		for _, sidecar := range sidecarImages {
			if sidecar.OwnerServer.Equals(r.PackageRef) {
				in = in.Computable(fmt.Sprintf("sidecar%d", sidecarIndex), sidecar.Image)
				sidecarIndex++
			}
		}

		images = append(images, compute.Map(tasks.Action("server.compute-images").Scope(r.PackageRef.PackageName()), in, compute.Output{},
			func(ctx context.Context, deps compute.Resolved) (ResolvedServerImages, error) {
				binary, _ := compute.GetDep(deps, r.Binary, "binary")

				result := ResolvedServerImages{
					PackageRef:     r.PackageRef,
					Binary:         binary.Value,
					PrebuiltBinary: binary.Completed.IsZero(),
				}

				if v, ok := compute.GetDep(deps, r.Config, "config"); ok {
					result.Config = v.Value
				}

				sidecarIndex := 0
				for _, sidecar := range sidecarImages {
					if sidecar.OwnerServer.Equals(r.PackageRef) {
						if v, ok := compute.GetDep(deps, sidecar.Image, fmt.Sprintf("sidecar%d", sidecarIndex)); ok {
							result.Sidecars = append(result.Sidecars, ResolvedSidecarImage{
								PackageRef: sidecar.PackageRef,
								Binary:     v.Value,
							})
						}
						sidecarIndex++
					}
				}

				return result, nil
			}))
	}

	return stack, images, nil
}

func prepareRunOpts(ctx context.Context, stack *stack.Stack, s provision.Server, imgs builtImage, out *runtime.ServerConfig) error {
	out.Server = s
	out.Image = imgs.Binary
	if imgs.Config.Repository != "" {
		out.ConfigImage = &imgs.Config
	}

	var err error
	if s.Integration() != nil {
		err = shared.PrepareRun(ctx, s, &out.ServerRunOpts)
	} else {
		err = languages.IntegrationFor(s.Framework()).PrepareRun(ctx, s, &out.ServerRunOpts)
	}
	if err != nil {
		return err
	}

	inputs := pkggraph.StartupInputs{
		Stack:         stack.Proto(),
		Server:        s.Proto(),
		ServerImage:   imgs.Binary.RepoAndDigest(),
		ServerRootAbs: s.Location.Abs(),
	}
	serverStartupPlan, err := s.Startup.EvalStartup(ctx, s.SealedContext(), inputs, nil)
	if err != nil {
		return err
	}
	merged, err := startup.ComputeConfig(ctx, s.SealedContext(), serverStartupPlan, stack.GetParsed(s.PackageName()).Deps, inputs)
	if err != nil {
		return err
	}

	out.Args = append(out.Args, merged.Args...)
	out.Env = append(out.Env, s.Proto().StaticEnv...)
	out.Env = append(out.Env, merged.Env...)

	return nil
}

func prepareContainerRunOpts(containers []*schema.SidecarContainer, imageIDs builtImages, sidecarCommands []sidecarPackage, out *[]runtime.SidecarRunOpts) error {
	for _, container := range containers {
		binRef := container.BinaryRef
		if binRef == nil {
			binRef = schema.NewPackageRef(schema.Name(container.Binary), "")
		}

		var sidecarPkg *sidecarPackage
		for _, ip := range sidecarCommands {
			if ip.PackageRef.Equals(binRef) {
				sidecarPkg = &ip
				break
			}
		}

		if sidecarPkg == nil {
			return fnerrors.InternalError("%s: missing a command", binRef)
		}

		img, ok := imageIDs.get(binRef)
		if !ok {
			return fnerrors.InternalError("%s: missing an image to run", binRef)
		}

		*out = append(*out, runtime.SidecarRunOpts{
			Name:       container.Name,
			PackageRef: binRef,
			ServerRunOpts: runtime.ServerRunOpts{
				Image:   img.Binary,
				Args:    container.Args,
				Command: sidecarPkg.Command,
			},
		})
	}
	return nil
}
