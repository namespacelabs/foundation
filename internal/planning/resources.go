// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import (
	"context"
	"sync"

	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/planning/invocation"
	"namespacelabs.dev/foundation/internal/planning/secrets"
	"namespacelabs.dev/foundation/internal/planning/tool"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/resources"
)

type resourcePlanner struct {
	eg      *executor.Executor
	secrets runtime.SecretSource
	mu      sync.Mutex
	state   map[string]*computedResource
}

type computedResource struct {
	resources []pkggraph.ResourceInstance
}

func newResourcePlanner(eg *executor.Executor, secrets runtime.SecretSource) *resourcePlanner {
	return &resourcePlanner{eg: eg, secrets: secrets, state: map[string]*computedResource{}}
}

func (rp *resourcePlanner) Complete() map[string][]pkggraph.ResourceInstance {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	m := map[string][]pkggraph.ResourceInstance{}
	for key, state := range rp.state {
		m[key] = state.resources
	}
	return m
}

func (rp *resourcePlanner) computeResource(sealedctx pkggraph.SealedContext, parentID string, res pkggraph.ResourceInstance, loadServer func(schema.PackageName)) error {
	rp.mu.Lock()
	defer rp.mu.Unlock()

	key := resources.JoinID(parentID, res.ResourceID)
	if _, ok := rp.state[key]; ok {
		return nil
	}

	if parsing.IsServerResource(res.Spec.Class.Ref) {
		if err := pkggraph.ValidateFoundation("runtime resources", parsing.Version_LibraryIntentsChanged, pkggraph.ModuleFromModules(sealedctx)); err != nil {
			return err
		}

		serverIntent := &schema.PackageRef{}
		if err := res.Spec.Intent.UnmarshalTo(serverIntent); err != nil {
			return fnerrors.InternalError("failed to unwrap Server")
		}

		loadServer(serverIntent.AsPackageName())
	}

	state := &computedResource{}
	rp.state[key] = state

	if res.Spec.Provider == nil {
		return nil
	}

	rp.eg.Go(func(ctx context.Context) error {
		resources, err := state.compute(ctx, rp.secrets, sealedctx, res.Spec.Intent, res.Spec.Provider)
		if err != nil {
			return err
		}

		for _, res := range resources {
			if err := rp.computeResource(sealedctx, parentID, res, loadServer); err != nil {
				return err
			}
		}

		rp.mu.Lock()
		defer rp.mu.Unlock()
		state.resources = resources
		return nil
	})

	return nil
}

func (st *computedResource) compute(ctx context.Context, secs runtime.SecretSource, sealedCtx pkggraph.SealedContext, intent *anypb.Any, provider *pkggraph.ResourceProvider) ([]pkggraph.ResourceInstance, error) {
	if provider == nil || provider.Spec.ResourcesFrom == nil {
		return nil, nil
	}

	inv, err := invocation.BuildAndPrepare(ctx, sealedCtx, sealedCtx, nil, provider.Spec.ResourcesFrom)
	if err != nil {
		return nil, fnerrors.InternalError("failed to compute invocation configuration: %w", err)
	}

	deferredResponse, err := tool.MakeInvocationNoInjections(ctx, sealedCtx,
		secrets.ScopeSecretsTo(secs, sealedCtx, nil),
		&tool.Definition{
			Source:     tool.Source{PackageName: schema.PackageName(provider.Spec.PackageName)},
			Invocation: inv,
		}, tool.InvokeProps{
			Event:          protocol.Lifecycle_PROVISION,
			ProvisionInput: []*anypb.Any{intent},
		})
	if err != nil {
		return nil, fnerrors.InternalError("resourcesFrom: failed to compute invocation: %w", err)
	}

	response, err := compute.GetValue(ctx, deferredResponse)
	if err != nil {
		return nil, fnerrors.InternalError("resourcesFrom: failed to invoke: %w", err)
	}

	if err := invocation.ValidateProviderReponse(response); err != nil {
		return nil, fnerrors.InternalError("resourcesFrom: %w", err)
	}

	r := response.ApplyResponse
	if r.OutputResourceInstance != nil {
		return nil, fnerrors.InternalError("resourcesFrom: response can't include resource instance")
	}

	pack := &schema.ResourcePack{}
	for _, x := range r.ComputedResourceInput {
		pack.ResourceInstance = append(pack.ResourceInstance, &schema.ResourceInstance{
			PackageName:          provider.Spec.PackageName,
			Name:                 x.Name,
			Class:                x.Class,
			Provider:             x.Provider,
			SerializedIntentJson: x.SerializedIntentJson,
		})
	}

	providerPkg, err := sealedCtx.LoadByName(ctx, schema.PackageName(provider.Spec.PackageName))
	if err != nil {
		return nil, fnerrors.InternalError("%s: missing provider package", provider.Spec.PackageName)
	}

	additionalResources, err := parsing.LoadResources(ctx, sealedCtx, providerPkg, provider.ProviderID, pack)
	if err != nil {
		return nil, fnerrors.InternalError("%s: failed to load computed resources: %w", provider.Spec.PackageName, err)
	}

	return additionalResources, nil
}
