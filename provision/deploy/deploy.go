// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/build/multiplatform"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/config"
	"namespacelabs.dev/foundation/provision/startup"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var AlsoComputeIngress = true

type StackOpts struct {
	BaseServerPort int32
}

type Opts struct {
	StackOpts
}

type images struct {
	Package schema.PackageName
	Binary  compute.Computable[oci.ImageID]
	Config  compute.Computable[oci.ImageID]
}

func PrepareDeployServers(ctx context.Context, env ops.Environment, focus []provision.Server, opts Opts, onStack func(*stack.Stack)) (compute.Computable[*Plan], error) {
	stack, err := stack.Compute(ctx, focus, stack.ProvisionOpts{PortBase: opts.BaseServerPort})
	if err != nil {
		return nil, err
	}

	if onStack != nil {
		onStack(stack)
	}

	return PrepareDeployStack(ctx, env, stack, focus)
}

func PrepareDeployStack(ctx context.Context, env ops.Environment, stack *stack.Stack, focus []provision.Server) (compute.Computable[*Plan], error) {
	buildID := provision.NewBuildID()

	def, err := prepareHandlerInvocations(ctx, env, stack)
	if err != nil {
		return nil, err
	}

	prepare, err := prepareBuildAndDeployment(ctx, env, focus, stack, def, buildID)
	if err != nil {
		return nil, err
	}

	g := &makeDeployGraph{prepare: prepare}
	if AlsoComputeIngress {
		g.ingress = PlanIngress(env, env.Proto(), stack.Proto())
	}

	return g, nil
}

type makeDeployGraph struct {
	prepare compute.Computable[prepareAndBuildResult]
	ingress compute.Computable[*ingressResult]

	compute.LocalScoped[*Plan]
}

type Plan struct {
	Deployer         *ops.Plan
	IngressFragments []*schema.IngressFragment
	Hints            []string // Optional messages to pass to the user.
}

func (m *makeDeployGraph) Action() *tasks.ActionEvent {
	return tasks.Action("deploy.make-graph")
}

func (m *makeDeployGraph) Inputs() *compute.In {
	in := compute.Inputs().Computable("prepare", m.prepare)
	if m.ingress != nil {
		in = in.Computable("ingress", m.ingress)
	}
	return in
}

func (m *makeDeployGraph) Output() compute.Output {
	return compute.Output{NonDeterministic: true}
}

func (m *makeDeployGraph) Compute(ctx context.Context, deps compute.Resolved) (*Plan, error) {
	pbr := compute.GetDepValue(deps, m.prepare, "prepare")

	g := ops.NewPlan()

	if err := g.Add(pbr.HandlerResult.Definitions...); err != nil {
		return nil, err
	}

	if err := g.Add(pbr.DeploymentState.Definitions()...); err != nil {
		return nil, err
	}

	plan := &Plan{
		Deployer: g,
		Hints:    pbr.DeploymentState.Hints(),
	}

	if ingress, ok := compute.GetDep(deps, m.ingress, "ingress"); ok {
		if err := g.Add(ingress.Value.DeploymentState.Definitions()...); err != nil {
			return nil, err
		}

		plan.IngressFragments = ingress.Value.Fragments
	}

	return plan, nil
}

func ComputeStack(ctx context.Context, server provision.Server, opts StackOpts, bid provision.BuildID) (*stack.Stack, error) {
	return stack.Compute(ctx, []provision.Server{server}, stack.ProvisionOpts{PortBase: opts.BaseServerPort})
}

func prepareHandlerInvocations(ctx context.Context, env ops.Environment, stack *stack.Stack) (c compute.Computable[*handlerResult], err error) {
	err = tasks.Action(runtime.TaskServerProvision).Scope(provision.ServerPackages(stack.Servers).PackageNames()...).Run(ctx,
		func(ctx context.Context) error {
			handlers, err := computeHandlers(ctx, stack)
			if err != nil {
				return err
			}

			// After we've computed the startup plans, issue the necessary provisioning calls.
			c, err = invokeHandlers(ctx, env, stack, handlers, protocol.Lifecycle_PROVISION)
			return err
		})
	return
}

type prepareAndBuildResult struct {
	HandlerResult   *handlerResult
	DeploymentState runtime.DeploymentState
}

type sidecarPackage struct {
	Package schema.PackageName
	Command []string
}

type builtImage struct {
	Package schema.PackageName
	Binary  oci.ImageID
	Config  oci.ImageID
}

type builtImages []builtImage

func (bi builtImages) get(pkg schema.PackageName) (builtImage, bool) {
	for _, p := range bi {
		if p.Package == pkg {
			return p, true
		}
	}
	return builtImage{}, false
}

func prepareBuildAndDeployment(ctx context.Context, env ops.Environment, servers []provision.Server, stack *stack.Stack, stackDef compute.Computable[*handlerResult], buildID provision.BuildID) (compute.Computable[prepareAndBuildResult], error) {
	var focus schema.PackageList
	for _, server := range servers {
		focus.Add(server.PackageName())
	}

	imgs, err := prepareServerImages(ctx, focus, stack, buildID)
	if err != nil {
		return nil, err
	}

	sidecarImages, err := prepareSidecarAndInitImages(ctx, stack, buildID)
	if err != nil {
		return nil, err
	}

	finalInputs := compute.Inputs()

	var sidecarCommands []sidecarPackage
	for pkg, v := range sidecarImages {
		// There's an assumption here that sidecar/init packages are non-overlapping with servers.
		imgs[pkg] = images{
			Package: pkg,
			Binary:  v.Image,
		}
		sidecarCommands = append(sidecarCommands, sidecarPackage{Package: pkg, Command: v.Command})
	}

	// Stable ordering.
	sort.Slice(sidecarCommands, func(i, j int) bool {
		return strings.Compare(sidecarCommands[i].Package.String(), sidecarCommands[j].Package.String()) < 0
	})

	// Ensure sidecarCommands are part of the cache key.
	finalInputs = finalInputs.JSON("sidecarCommands", sidecarCommands)

	// A two-layer graph is created here: the first layer depends on all the server binaries,
	// while the second layer depends on all config images (if specified), plus depending on
	// the outcome of invoking all handlers, and then the outcome of all server images. This
	// allows all builds and invocations to occur in parallel.

	binaryInputs := compute.Inputs()
	for pkg, img := range imgs {
		if img.Binary != nil {
			binaryInputs = binaryInputs.Computable(fmt.Sprintf("%s:binary", pkg), img.Binary)
		}
		if img.Config != nil {
			binaryInputs = binaryInputs.Computable(fmt.Sprintf("%s:config", pkg), img.Config)
		}
	}

	imageIDs := compute.Map(tasks.Action(runtime.TaskServerBuild),
		binaryInputs, compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (builtImages, error) {
			var built builtImages

			for pkg := range imgs {
				img, ok := compute.GetDepWithType[oci.ImageID](deps, fmt.Sprintf("%s:binary", pkg))
				if !ok {
					return nil, fnerrors.InternalError("server image missing")
				}

				b := builtImage{
					Package: pkg,
					Binary:  img.Value,
				}

				if v, ok := compute.GetDepWithType[oci.ImageID](deps, fmt.Sprintf("%s:config", pkg)); ok {
					b.Config = v.Value
				}

				built = append(built, b)
			}

			// Stable ordering.
			sort.Slice(built, func(i, j int) bool {
				return strings.Compare(built[i].Package.String(), built[j].Package.String()) < 0
			})

			return built, nil
		})

	c1 := compute.Map(
		tasks.Action(runtime.TaskServerProvision).
			Scope(provision.ServerPackages(stack.Servers).PackageNames()...),
		finalInputs.Computable("images", imageIDs).Computable("stackAndDefs", stackDef),
		compute.Output{},
		func(ctx context.Context, deps compute.Resolved) (prepareAndBuildResult, error) {
			imageIDs := compute.GetDepValue(deps, imageIDs, "images")
			handlerR := compute.GetDepValue(deps, stackDef, "stackAndDefs")
			stack := handlerR.Stack

			// And finally compute the startup plan of each server in the stack, passing in the id of the
			// images we just built.
			var serverRuns []runtime.ServerConfig
			for k, s := range stack.Servers {
				imgs, ok := imageIDs.get(s.PackageName())
				if !ok {
					return prepareAndBuildResult{}, fnerrors.InternalError("%s: missing an image to run", s.PackageName())
				}

				var run runtime.ServerConfig
				if err := prepareRunOpts(ctx, stack, s, imgs, &run); err != nil {
					return prepareAndBuildResult{}, err
				}

				for _, dep := range stack.ParsedServers[k].Deps {
					if err := prepareContainerRunOpts(dep.ProvisionPlan.Sidecars, imageIDs, sidecarCommands, &run.Sidecars); err != nil {
						return prepareAndBuildResult{}, err
					}
					if err := prepareContainerRunOpts(dep.ProvisionPlan.Inits, imageIDs, sidecarCommands, &run.Inits); err != nil {
						return prepareAndBuildResult{}, err
					}
				}

				run.Extensions = handlerR.ServerExtensions[s.PackageName()]

				serverRuns = append(serverRuns, run)
			}

			deployment, err := runtime.For(ctx, env).PlanDeployment(ctx, runtime.Deployment{
				BuildID: buildID,
				Focus:   focus,
				Stack:   stack.Proto(),
				Servers: serverRuns,
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

func prepareServerImages(ctx context.Context, focus schema.PackageList, stack *stack.Stack, buildID provision.BuildID) (map[schema.PackageName]images, error) {
	imageMap := map[schema.PackageName]images{}

	for _, srv := range stack.Servers {
		var images images

		spec, err := languages.IntegrationFor(srv.Framework()).PrepareBuild(ctx, stack, srv, focus.Includes(srv.PackageName()))
		if err != nil {
			return nil, err
		}

		if imgid, ok := build.IsPrebuilt(spec); ok {
			images.Binary = build.Prebuilt(imgid)
		} else {
			p, err := makePlan(ctx, srv, spec)
			if err != nil {
				return nil, err
			}

			name, err := registry.AllocateName(ctx, srv.Env(), srv.PackageName(), buildID)
			if err != nil {
				return nil, err
			}

			// Leave a hint to where we're pushing to, in case the builder can
			// use that information for optimization purposes. This may be
			// replaced with a graph optimization pass in the future.
			p.PublishName = name

			bin, err := multiplatform.PrepareMultiPlatformImage(ctx, srv.Env(), p)
			if err != nil {
				return nil, err
			}

			images.Binary = oci.PublishResolvable(name, bin)
		}

		// In production builds, also build a "config image" which includes both the processed
		// stack at the time of evaluation of the target image and deployment, but also the
		// source configuration files used to compute a startup configuration, so it can be re-
		// evaluated on a need basis.
		if focus.Includes(srv.PackageName()) {
			switch srv.Env().Proto().GetPurpose() {
			case schema.Environment_DEVELOPMENT, schema.Environment_PRODUCTION:
				configImage := prepareConfigImage(ctx, srv, stack)

				cfgtag, err := registry.AllocateName(ctx, srv.Env(), srv.PackageName(), config.MakeConfigTag(buildID))
				if err != nil {
					return nil, err
				}

				images.Config = oci.PublishImage(cfgtag, configImage)
			}
		}

		imageMap[srv.PackageName()] = images
	}

	return imageMap, nil
}

type containerImage struct {
	Image   compute.Computable[oci.ImageID]
	Command []string
}

func prepareSidecarAndInitImages(ctx context.Context, stack *stack.Stack, buildID provision.BuildID) (map[schema.PackageName]containerImage, error) {
	res := map[schema.PackageName]containerImage{}
	for k, srv := range stack.Servers {
		platforms, err := runtime.For(ctx, srv.Env()).TargetPlatforms(ctx)
		if err != nil {
			return nil, err
		}

		for _, dep := range stack.ParsedServers[k].Deps {
			containers := append([]*schema.SidecarContainer{}, dep.ProvisionPlan.Sidecars...)
			containers = append(containers, dep.ProvisionPlan.Inits...)

			for _, container := range containers {
				pkgname := schema.PackageName(container.Binary)
				bin, err := srv.Env().LoadByName(ctx, pkgname)
				if err != nil {
					return nil, err
				}

				prepared, err := binary.Plan(ctx, bin,
					binary.BuildImageOpts{
						UsePrebuilts: true,
						Platforms:    platforms,
					})
				if err != nil {
					return nil, err
				}

				image, err := prepared.Image(ctx, srv.Env())
				if err != nil {
					return nil, err
				}

				tag, err := registry.AllocateName(ctx, srv.Env(), bin.PackageName(), buildID)
				if err != nil {
					return nil, err
				}

				res[pkgname] = containerImage{
					Image:   oci.PublishResolvable(tag, image),
					Command: prepared.Command,
				}
			}
		}
	}
	return res, nil
}

func ComputeStackAndImages(ctx context.Context, servers []provision.Server, opts Opts) (*stack.Stack, []compute.Computable[oci.ImageID], error) {
	bid := provision.NewBuildID()

	stack, err := stack.Compute(ctx, servers, stack.ProvisionOpts{PortBase: opts.BaseServerPort})
	if err != nil {
		return nil, nil, err
	}

	m, err := prepareServerImages(ctx, provision.ServerPackages(servers), stack, bid)
	if err != nil {
		return nil, nil, err
	}

	var images []compute.Computable[oci.ImageID]
	for _, r := range m {
		if r.Binary != nil {
			images = append(images, r.Binary)
		}
		if r.Config != nil {
			images = append(images, r.Config)
		}
	}

	return stack, images, nil
}

func prepareRunOpts(ctx context.Context, stack *stack.Stack, s provision.Server, imgs builtImage, out *runtime.ServerConfig) error {
	out.Server = s
	out.Image = imgs.Binary
	if imgs.Config.Repository != "" {
		out.ConfigImage = &imgs.Config
	}

	if err := languages.IntegrationFor(s.Framework()).PrepareRun(ctx, s, &out.ServerRunOpts); err != nil {
		return err
	}

	merged, err := startup.ComputeConfig(ctx, s.Env(), stack.GetParsed(s.PackageName()), frontend.StartupInputs{
		Stack:         stack.Proto(),
		Server:        s.Proto(),
		ServerImage:   imgs.Binary.RepoAndDigest(),
		ServerRootAbs: s.Location.Abs(),
	})
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
		pkg := schema.PackageName(container.Binary)

		var sidecarPkg *sidecarPackage
		for _, ip := range sidecarCommands {
			if ip.Package == pkg {
				sidecarPkg = &ip
				break
			}
		}

		if sidecarPkg == nil {
			return fnerrors.InternalError("%s: missing a command", pkg)
		}

		imgs, ok := imageIDs.get(pkg)
		if !ok {
			return fnerrors.InternalError("%s: missing an image to run", pkg)
		}

		*out = append(*out, runtime.SidecarRunOpts{
			PackageName: pkg,
			ServerRunOpts: runtime.ServerRunOpts{
				Image:   imgs.Binary,
				Args:    container.Args,
				Command: sidecarPkg.Command,
			},
		})
	}
	return nil
}
