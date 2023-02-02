// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"encoding/json"
	"io/fs"
	"strings"

	"github.com/moby/buildkit/client/llb"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/rpcerrors/multierr"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	protos2 "namespacelabs.dev/foundation/internal/codegen/protos"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/llbutil"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/planning/invocation"
	"namespacelabs.dev/foundation/internal/planning/secrets"
	"namespacelabs.dev/foundation/internal/planning/tool"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/runtime/tools"
	is "namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
	"namespacelabs.dev/foundation/std/tasks"
)

type resourcePlan struct {
	ResourceList resourceList

	PlannedResources     []plannedResource
	ExecutionInvocations []*schema.SerializedInvocation
	Secrets              []runtime.SecretResourceDependency
}

type plannedResource struct {
	runtime.ComputedResource
	Invocations []*schema.SerializedInvocation
}

type resourcePlanInvocation struct {
	Env                  pkggraph.SealedContext
	Secrets              is.GroundedSecrets
	Source               schema.PackageName
	Resource             *resourceInstance
	ResourceSource       *schema.ResourceInstance
	ResourceClass        *schema.ResourceClass
	Invocation           *invocation.Invocation
	Intent               *anypb.Any
	SerializedIntentJson []byte
	InstanceTypeSource   *protos2.FileDescriptorSetAndDeps
	Image                oci.ResolvableImage
}

func planResources(ctx context.Context, planner planning.Planner, stack *planning.StackWithIngress, rp resourceList) (*resourcePlan, error) {
	platforms, err := planner.Runtime.TargetPlatforms(ctx)
	if err != nil {
		return nil, err
	}

	rlist := rp.Resources()

	var executionInvocations []*InvokeResourceProvider
	var planningInvocations []resourcePlanInvocation
	var imageIDs []compute.Computable[oci.ImageID]

	plan := &resourcePlan{
		ResourceList: rp,
	}

	for _, resource := range rlist {
		if len(resource.ParentContexts) == 0 {
			return nil, fnerrors.InternalError("%s: resource is missing a context", resource.ID)
		}

		// Any of the contexts should be valid to load the binary, as all of them refer to this resources.
		sealedCtx := resource.ParentContexts[0]

		provider := resource.Provider.Spec

		switch {
		case parsing.IsSecretResource(resource.Class.Ref):
			if len(resource.Dependencies) > 0 {
				return nil, fnerrors.New("runtime secrets don't support dependencies")
			}

			// Nothing to do
			continue

		case parsing.IsServerResource(resource.Class.Ref):
			if len(resource.Dependencies) > 0 {
				return nil, fnerrors.New("runtime servers don't support dependencies")
			}

			if err := pkggraph.ValidateFoundation("runtime resources", parsing.Version_LibraryIntentsChanged, pkggraph.ModuleFromModules(sealedCtx)); err != nil {
				return nil, err
			}

			serverIntent := &schema.PackageRef{}
			if err := proto.Unmarshal(resource.Intent.Value, serverIntent); err != nil {
				return nil, fnerrors.InternalError("failed to unmarshal serverintent: %w", err)
			}

			target, has := stack.Get(serverIntent.AsPackageName())
			if !has {
				return nil, fnerrors.InternalError("%s: target server is not in the stack", serverIntent.PackageName)
			}

			si := &schema.SerializedInvocation{
				Description: "Capture Runtime Config",
				Order: &schema.ScheduleOrder{
					SchedCategory: []string{
						resources.ResourceInstanceCategory(resource.ID),
					},
					SchedAfterCategory: []string{
						runtime.OwnedByDeployable(target.Proto()),
					},
				},
			}

			wrapped, err := anypb.New(&resources.OpCaptureServerConfig{
				ResourceInstanceId: resource.ID,
				ServerConfig:       makeServerConfig(stack, target, sealedCtx.Environment()),
				Deployable:         runtime.DeployableToProto(target.Proto()),
			})
			if err != nil {
				return nil, err
			}

			si.Impl = wrapped

			plan.ExecutionInvocations = append(plan.ExecutionInvocations, si)

		case provider.PrepareWith != nil:
			var errs []error
			if len(resource.Dependencies) > 0 {
				errs = append(errs, fnerrors.New("providers with prepareWith don't support dependencies yet"))
			}

			if len(resource.Secrets) > 0 {
				errs = append(errs, fnerrors.New("providers with prepareWith don't support secrets yet"))
			}

			inv, err := invocation.BuildAndPrepare(ctx, sealedCtx, sealedCtx, nil, provider.PrepareWith)
			if err != nil {
				errs = append(errs, err)
			}

			if err := multierr.New(errs...); err != nil {
				return nil, err
			}

			planningInvocations = append(planningInvocations, resourcePlanInvocation{
				Env:                  sealedCtx,
				Secrets:              secrets.ScopeSecretsTo(planner.Secrets, sealedCtx, nil),
				Source:               schema.PackageName(resource.Source.PackageName),
				Resource:             resource,
				ResourceSource:       resource.Source,
				ResourceClass:        resource.Class.Source,
				Invocation:           inv,
				Intent:               resource.Intent,
				SerializedIntentJson: resource.JSONSerializedIntent,
				InstanceTypeSource:   resource.Class.InstanceType.Sources,
			})

		case provider.InitializedWith != nil:
			initializer := provider.InitializedWith
			if initializer.RequiresKeys || initializer.Snapshots != nil || initializer.Inject != nil {
				return nil, fnerrors.InternalError("bad resource provider initialization: unsupported inputs")
			}

			pkg, bin, err := pkggraph.LoadBinary(ctx, sealedCtx, initializer.BinaryRef)
			if err != nil {
				return nil, err
			}

			prepared, err := binary.PlanBinary(ctx, sealedCtx, sealedCtx, pkg.Location, bin, assets.AvailableBuildAssets{}, binary.BuildImageOpts{
				UsePrebuilts: true,
				Platforms:    platforms,
			})
			if err != nil {
				return nil, err
			}

			config, err := invocation.MergePreparedConfig(prepared, initializer)
			if err != nil {
				return nil, err
			}

			poster, err := ensureImage(ctx, sealedCtx, planner.Registry, prepared.Plan)
			if err != nil {
				return nil, err
			}

			imageIDs = append(imageIDs, poster.ImageID)

			p := &InvokeResourceProvider{
				ResourceInstanceId:   resource.ID,
				BinaryRef:            initializer.BinaryRef,
				BinaryLabels:         prepared.Labels,
				ResourceClass:        resource.Class.Source,
				ResourceProvider:     provider,
				InstanceTypeSource:   resource.Class.InstanceType.Sources,
				ResourceDependencies: resource.Dependencies,
				SecretResources:      resource.Secrets,
			}
			p.SealedContext = sealedCtx
			p.BinaryConfig = config
			p.SerializedIntentJson = resource.JSONSerializedIntent

			executionInvocations = append(executionInvocations, p)

		default:
			return nil, fnerrors.InternalError("%s: an initializer is missing", resource.ID)
		}
	}

	builtExecutionImages, err := compute.GetValue(ctx, compute.Collect(tasks.Action("resources.build-execution-images"), imageIDs...))
	if err != nil {
		return nil, err
	}

	for k, invocation := range executionInvocations {
		invocation.BinaryImageId = builtExecutionImages[k].Value

		theseOps, err := PlanResourceProviderInvocation(ctx, planner.Secrets, planner.Runtime, invocation)
		if err != nil {
			return nil, err
		}

		plan.ExecutionInvocations = append(plan.ExecutionInvocations, theseOps...)
	}

	var invocationResponses []compute.Computable[plannedResource]
	var errs []error
	for _, planned := range planningInvocations {
		if supportsDirectInvocation(planned.Invocation.BinaryLabels) {
			p, err := directPlanningInvocation(ctx, planned)
			if err != nil {
				errs = append(errs, err)
			} else {
				invocationResponses = append(invocationResponses, p)
			}
			continue
		}

		source, err := anypb.New(&protocol.ResourceInstance{
			ResourceInstanceId: planned.Resource.ID,
			ResourceInstance:   planned.ResourceSource,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}

		inv, err := tool.MakeInvocationNoInjections(ctx, planned.Env, planned.Secrets, &tool.Definition{
			Source:     tool.Source{PackageName: planned.Source},
			Invocation: planned.Invocation,
		}, tool.InvokeProps{
			Event:          protocol.Lifecycle_PROVISION,
			ProvisionInput: []*anypb.Any{planned.Intent, source},
		})
		if err != nil {
			errs = append(errs, err)
		} else {
			planned := planned // Close planned.
			invocationResponses = append(invocationResponses, compute.Transform("validate", inv, func(ctx context.Context, response *protocol.ToolResponse) (plannedResource, error) {
				if err := invocation.ValidateProviderReponse(response); err != nil {
					return plannedResource{}, err
				}

				r := response.ApplyResponse

				if len(r.ComputedResourceInput) > 0 {
					return plannedResource{}, fnerrors.InternalError("prepareWith response can't include computed resourced")
				}

				return plannedResource{
					ComputedResource: runtime.ComputedResource{
						ResourceInstanceID:     planned.Resource.ID,
						InstanceType:           planned.ResourceClass.InstanceType,
						InstanceSerializedJSON: r.OutputResourceInstanceSerializedJson,
					},
					Invocations: r.Invocation,
				}, nil
			}))
		}
	}

	if err := multierr.New(errs...); err != nil {
		return nil, err
	}

	responses, err := compute.GetValue(ctx, compute.Collect(tasks.Action("resources.invoke-providers"), invocationResponses...))
	if err != nil {
		return nil, err
	}

	for _, raw := range responses {
		plan.PlannedResources = append(plan.PlannedResources, raw.Value)
	}

	return plan, nil
}

func directPlanningInvocation(ctx context.Context, planned resourcePlanInvocation) (compute.Computable[plannedResource], error) {
	invocation := planned.Invocation

	args, err := planProviderArgs(ctx, providerArgsInput{
		SealedContext:        planned.Env,
		SerializedIntentJson: planned.SerializedIntentJson,
		BinaryConfig:         planned.Invocation.Config,
	})
	if err != nil {
		return nil, err
	}

	args = append(args, "--planning_output=/out/output.json")

	opts := rtypes.RunBinaryOpts{
		Command:    invocation.Config.Command,
		Args:       append(slices.Clone(invocation.Config.Args), args...),
		WorkingDir: invocation.Config.WorkingDir,
		Env:        invocation.Config.Env,
	}

	cli, err := invocation.Buildkit.MakeClient(ctx)
	if err != nil {
		return nil, err
	}

	ximage := oci.ResolveImagePlatform(invocation.Image, cli.BuildkitOpts().HostPlatform)
	state := makeState(cli, planned.Source, ximage, opts)
	files := tools.InvokeOnBuildkit0(cli, planned.Secrets, planned.Source, state)

	return compute.Transform("parse json", files, func(ctx context.Context, fsys fs.FS) (plannedResource, error) {
		instanceData, err := fs.ReadFile(fsys, "instance.json")
		if err != nil {
			return plannedResource{}, err
		}

		var output PlanningOutput
		if err := json.Unmarshal(instanceData, &output); err != nil {
			return plannedResource{}, fnerrors.BadInputError("failed to decode output: %w", err)
		}

		return plannedResource{
			ComputedResource: runtime.ComputedResource{
				ResourceInstanceID:     planned.Resource.ID,
				InstanceType:           planned.ResourceClass.InstanceType,
				InstanceSerializedJSON: []byte(output.InstanceSerializedJSON),
			},
		}, nil
	}), nil
}

type PlanningOutput struct {
	InstanceSerializedJSON string `json:"instance_json"`
}

func makeState(c *buildkit.GatewayClient, pkg schema.PackageName, image compute.Computable[oci.Image], opts rtypes.RunBinaryOpts) compute.Computable[*buildkit.Input] {
	return compute.Transform("make-request", tools.EnsureCached(image), func(ctx context.Context, image oci.Image) (*buildkit.Input, error) {
		d, err := image.Digest()
		if err != nil {
			return nil, err
		}

		tasks.Attachments(ctx).AddResult("ref", d.String())

		if !c.BuildkitOpts().SupportsCanonicalBuilds {
			return nil, fnerrors.InvocationError("buildkit", "the target buildkit does not have the required capabilities (ocilayout input), please upgrade")
		}

		base := llbutil.OCILayout(d, llb.WithCustomNamef("%s: base image (%s)", pkg, d))

		args := append(slices.Clone(opts.Command), opts.Args...)

		runOpts := []llb.RunOption{llb.ReadonlyRootFS(), llb.Network(llb.NetModeNone), llb.Args(args)}
		if opts.WorkingDir != "" {
			runOpts = append(runOpts, llb.Dir(opts.WorkingDir))
		}

		var secrets []*schema.PackageRef
		for _, env := range opts.Env {
			if env.FromSecretRef != nil {
				runOpts = append(runOpts, llb.AddSecret(env.Name, llb.SecretAsEnv(true), llb.SecretID(env.FromSecretRef.Canonical())))
				secrets = append(secrets, env.FromSecretRef)
				continue
			}

			if env.ExperimentalFromSecret != "" || env.ExperimentalFromDownwardsFieldPath != "" || env.FromServiceEndpoint != nil || env.FromServiceIngress != nil || env.FromResourceField != nil {
				return nil, fnerrors.New("invocation: only support environment variables with static values")
			}

			runOpts = append(runOpts, llb.AddEnv(env.Name, env.Value))
		}

		run := base.Run(runOpts...)

		out := run.AddMount("/out", llb.Scratch())
		return &buildkit.Input{State: out, Secrets: secrets}, nil
	})
}

func supportsDirectInvocation(labels []*schema.Label) bool {
	for _, kv := range labels {
		if kv.Name == "namespace.so/binary-protocol" {
			return kv.Value == "inlineprovider.namespace.so/v1alpha1"
		}
	}
	return false
}

type resourceList struct {
	resources         map[string]*resourceInstance
	perOwnerResources ResourceMap
}

type ResourceMap map[string]ownedResourceInstances // The key is a canonical PackageRef.

type resourceInstance struct {
	ParentContexts []pkggraph.SealedContext

	ID                   string
	Source               *schema.ResourceInstance
	Class                pkggraph.ResourceClass
	Provider             *pkggraph.ResourceProvider
	Intent               *anypb.Any
	JSONSerializedIntent []byte
	Dependencies         []*resources.ResourceDependency
	PlannedDependencies  []*resources.ResourceDependency
	Secrets              []runtime.SecretResourceDependency
}

type ownedResourceInstances struct {
	Dependencies        []*resources.ResourceDependency
	PlannedDependencies []*resources.ResourceDependency
	Secrets             []runtime.SecretResourceDependency
}

func (rp *resourceList) Resources() []*resourceInstance {
	var resources []*resourceInstance
	for _, res := range rp.resources {
		resources = append(resources, res)
	}
	slices.SortFunc(resources, func(a, b *resourceInstance) bool {
		return strings.Compare(a.ID, b.ID) < 0
	})
	return resources
}

type resourceOwner interface {
	SealedContext() pkggraph.SealedContext
	PackageRef() *schema.PackageRef
}

func (rp *resourceList) checkAddOwnedResources(ctx context.Context, stack *planning.StackWithIngress, owner resourceOwner, instances []pkggraph.ResourceInstance) error {
	var instance resourceInstance

	if err := rp.checkAddTo(ctx, stack, owner.SealedContext(), "", instances, &instance); err != nil {
		return err
	}

	if rp.perOwnerResources == nil {
		rp.perOwnerResources = ResourceMap{}
	}

	rp.perOwnerResources[owner.PackageRef().Canonical()] = ownedResourceInstances{
		instance.Dependencies,
		instance.PlannedDependencies,
		instance.Secrets,
	}

	return nil
}

func (rp *resourceList) checkAddResource(ctx context.Context, stack *planning.StackWithIngress, sealedCtx pkggraph.SealedContext, resourceID string, resource pkggraph.ResourceSpec) error {
	if existing, has := rp.resources[resourceID]; has {
		existing.ParentContexts = append(existing.ParentContexts, sealedCtx)
		return nil
	}

	if rp.resources == nil {
		rp.resources = map[string]*resourceInstance{}
	}

	instance := resourceInstance{
		ParentContexts: []pkggraph.SealedContext{sealedCtx},
		ID:             resourceID,
		Source:         resource.Source,
		Class:          resource.Class,
		Provider:       resource.Provider,
		Intent:         resource.Intent,
	}

	if instance.Intent != nil {
		out := dynamicpb.NewMessage(resource.IntentType.Descriptor).Interface()

		if err := proto.Unmarshal(instance.Intent.Value, out); err != nil {
			return fnerrors.InternalError("%s: failed to unmarshal intent: %w", resourceID, err)
		}

		// json.Marshal is not capable of serializing a dynamicpb.
		serialized, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(out)
		if err != nil {
			return fnerrors.InternalError("%s: failed to marshal intent to json: %w", resourceID, err)
		}

		instance.JSONSerializedIntent = serialized
	}

	var inputs []pkggraph.ResourceInstance
	if resource.Provider != nil {
		inputs = append(inputs, resource.Provider.Resources...)
	}

	inputs = append(inputs, stack.GetComputedResources(resourceID)...)
	inputs = append(inputs, resource.ResourceInputs...)

	if err := rp.checkAddTo(ctx, stack, sealedCtx, resourceID, inputs, &instance); err != nil {
		return err
	}

	rp.resources[resourceID] = &instance
	return nil
}

func (rp *resourceList) checkAddTo(ctx context.Context, stack *planning.StackWithIngress, sealedCtx pkggraph.SealedContext, parentID string, inputs []pkggraph.ResourceInstance, instance *resourceInstance) error {
	regular, secrets, err := splitRegularAndSecretResources(ctx, inputs)
	if err != nil {
		return err
	}

	// Add static resources required by providers.
	var errs []error
	for _, res := range regular {
		scopedID := resources.JoinID(parentID, res.ResourceID)

		if err := rp.checkAddResource(ctx, stack, sealedCtx, scopedID, res.Spec); err != nil {
			errs = append(errs, err)
		} else {
			dep := &resources.ResourceDependency{
				ResourceRef:        res.ResourceRef,
				ResourceClass:      res.Spec.Class.Ref,
				ResourceInstanceId: scopedID,
			}

			if res.Spec.Provider != nil && res.Spec.Provider.Spec.GetPrepareWith() != nil {
				instance.PlannedDependencies = append(instance.PlannedDependencies, dep)
			} else {
				instance.Dependencies = append(instance.Dependencies, dep)
			}
		}
	}

	instance.Secrets = secrets
	return multierr.New(errs...)
}

func splitRegularAndSecretResources(ctx context.Context, inputs []pkggraph.ResourceInstance) ([]pkggraph.ResourceInstance, []runtime.SecretResourceDependency, error) {
	var regularDependencies []pkggraph.ResourceInstance
	var secs []runtime.SecretResourceDependency
	for _, dep := range inputs {
		if parsing.IsSecretResource(dep.Spec.Class.Ref) {
			ref := &schema.PackageRef{}
			if err := proto.Unmarshal(dep.Spec.Intent.Value, ref); err != nil {
				return nil, nil, fnerrors.InternalError("failed to unmarshal serverintent: %w", err)
			}

			secs = append(secs, runtime.SecretResourceDependency{
				SecretRef:   ref,
				ResourceRef: dep.ResourceRef,
			})
		} else {
			regularDependencies = append(regularDependencies, dep)
		}
	}

	return regularDependencies, secs, nil
}
