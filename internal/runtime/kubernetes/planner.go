// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"bytes"
	"context"
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"sigs.k8s.io/yaml"
)

type Planner struct {
	Configuration cfg.Configuration

	fetchSystemInfo FetchSystemInfoFunc
	ingress         kubedef.IngressClass
	underlying      *Cluster
	target          clusterTarget
	registry        registry.Manager
}

var _ runtime.Planner = Planner{}

type FetchSystemInfoFunc func(context.Context) (*kubedef.SystemInfo, error)

type NewClusterOpts struct {
	FetchSystemInfo         FetchSystemInfoFunc
	SupportedIngressClasses []string
}

func NewPlanner(ctx context.Context, env cfg.Context, fetch FetchSystemInfoFunc, ingressClass kubedef.IngressClass) (Planner, error) {
	registry, err := registry.GetRegistry(ctx, env)
	if err != nil {
		return Planner{}, err
	}

	return NewPlannerWithRegistry(env, registry, fetch, ingressClass), nil
}

func NewPlannerWithRegistry(env cfg.Context, registry registry.Manager, fetch FetchSystemInfoFunc, ingressClass kubedef.IngressClass) Planner {
	return Planner{
		Configuration:   env.Configuration(),
		fetchSystemInfo: fetch,
		ingress:         ingressClass,
		target:          newTarget(env),
		registry:        registry,
	}
}

func (r Planner) Ingress() runtime.IngressClass {
	return r.ingress
}

func (r Planner) PlanDeployment(ctx context.Context, d runtime.DeploymentSpec) (*runtime.DeploymentPlan, error) {
	return planDeployment(ctx, r.target, d)
}

func (r Planner) PlanIngress(ctx context.Context, stack *fnschema.Stack, allFragments []*fnschema.IngressFragment) (*runtime.DeploymentPlan, error) {
	return planIngress(ctx, r.ingress, r.target, stack, allFragments)
}

func (r Planner) KubernetesNamespace() string { return r.target.namespace }

func (r Planner) PrepareProvision(ctx context.Context) (*rtypes.ProvisionProps, error) {
	systemInfo, err := r.fetchSystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	return PrepareProvisionWith(r.target.env, r.target.namespace, systemInfo)
}

func (r Planner) ComputeBaseNaming(*fnschema.Naming) (*fnschema.ComputedNaming, error) {
	// The default kubernetes integration has no assumptions regarding how ingress names are allocated.
	return nil, nil
}

func (r Planner) TargetPlatforms(ctx context.Context) ([]specs.Platform, error) {
	if !UseNodePlatformsForProduction && r.target.env.Purpose == fnschema.Environment_PRODUCTION {
		return parsePlatforms(ProductionPlatforms)
	}

	systemInfo, err := r.fetchSystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	return parsePlatforms(systemInfo.NodePlatform)
}

func (r Planner) Registry() registry.Manager {
	return r.registry
}

func (r Planner) EnsureClusterNamespace(ctx context.Context) (runtime.ClusterNamespace, error) {
	return r.ClusterNamespaceFor(r.underlying, r.underlying), nil
}

func (r Planner) ClusterNamespaceFor(parent runtime.Cluster, underlying *Cluster) *ClusterNamespace {
	return &ClusterNamespace{parent: parent, underlying: underlying, target: r.target}
}

func planDeployment(ctx context.Context, target clusterTarget, d runtime.DeploymentSpec) (*runtime.DeploymentPlan, error) {
	var state runtime.DeploymentPlan

	for _, deployable := range d.Specs {
		var singleState serverRunState

		if err := prepareDeployment(ctx, target, deployable, deployOpts{
			secrets: deployable.Secrets,
		}, &singleState); err != nil {
			return nil, err
		}

		// XXX verify we've consumed all endpoints.
		for _, endpoint := range deployable.Endpoints {
			if err := deployEndpoint(ctx, target, deployable, endpoint, &singleState); err != nil {
				return nil, err
			}
		}

		if at := tasks.Attachments(ctx); deployable.GetPackageRef().GetPackageName() != "" {
			output := &bytes.Buffer{}
			count := 0
			for _, decl := range singleState.operations {
				if count > 0 {
					fmt.Fprintln(output, "---")
				}

				resource := decl.AppliedResource()
				if resource == nil {
					continue
				}

				count++
				b, err := yaml.Marshal(resource)
				if err == nil {
					fmt.Fprintf(output, "%s\n", b)
					// XXX ignoring errors
				}
			}

			at.Attach(tasks.Output(fmt.Sprintf("%s.k8s-decl.yaml", deployable.GetPackageRef().GetPackageName()), "application/yaml"), output.Bytes())
		}

		for _, apply := range singleState.operations {
			def, err := apply.ToDefinition(deployable.GetPackageRef().AsPackageName())
			if err != nil {
				return nil, err
			}
			state.Definitions = append(state.Definitions, def)
		}
	}

	if !target.env.GetEphemeral() {
		// TODO skip cleanup from CLI when orchestrator does it.
		cleanup, err := anypb.New(&kubedef.OpCleanupRuntimeConfig{
			Namespace: target.namespace,
			CheckPods: deployAsPods(target.env),
		})
		if err != nil {
			return nil, fnerrors.InternalError("failed to serialize cleanup: %w", err)
		}

		state.Definitions = append(state.Definitions, &fnschema.SerializedInvocation{
			Description: "Kubernetes: cleanup unused resources",
			Impl:        cleanup,
		})
	}

	state.NamespaceReference = target.namespace

	return &state, nil
}
