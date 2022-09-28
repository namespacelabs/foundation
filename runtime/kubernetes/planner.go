// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"bytes"
	"context"
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/rtypes"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
	"sigs.k8s.io/yaml"
)

type Planner struct {
	fetchSystemInfo func(context.Context) (*kubedef.SystemInfo, error)
	target          clusterTarget
}

var _ runtime.Namespace = Planner{}
var _ runtime.Planner = Planner{}

func NewPlanner(env planning.Context, fetchSystemInfo func(context.Context) (*kubedef.SystemInfo, error)) Planner {
	return Planner{fetchSystemInfo: fetchSystemInfo, target: newTarget(env)}
}

func (r Planner) UniqueID() string {
	return fmt.Sprintf("kubernetes:%s", r.target.namespace)
}

func (r Planner) Planner() runtime.Planner {
	return r
}

func (r Planner) PlanDeployment(ctx context.Context, d runtime.DeploymentSpec) (*runtime.DeploymentPlan, error) {
	return planDeployment(ctx, r.target, d)
}

func (r Planner) PlanIngress(ctx context.Context, stack *fnschema.Stack, allFragments []*fnschema.IngressFragment) (*runtime.DeploymentPlan, error) {
	return planIngress(ctx, r.target, stack, allFragments)
}

func (r Planner) Namespace() runtime.Namespace {
	return r
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

func planDeployment(ctx context.Context, target clusterTarget, d runtime.DeploymentSpec) (*runtime.DeploymentPlan, error) {
	var state runtime.DeploymentPlan
	deployOpts := deployOpts{
		secrets: d.Secrets,
	}

	for _, deployable := range d.Specs {
		var singleState serverRunState

		if err := prepareDeployment(ctx, target, deployable, deployable.InternalEndpoints, deployOpts, &singleState); err != nil {
			return nil, err
		}

		// XXX verify we've consumed all endpoints.
		for _, endpoint := range deployable.Endpoints {
			if err := deployEndpoint(ctx, target, deployable, endpoint, &singleState); err != nil {
				return nil, err
			}
		}

		if at := tasks.Attachments(ctx); deployable.PackageName != "" && at.IsStoring() {
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

			at.Attach(tasks.Output(fmt.Sprintf("%s.k8s-decl.yaml", deployable.PackageName), "application/yaml"), output.Bytes())
		}

		for _, apply := range singleState.operations {
			def, err := apply.ToDefinition(deployable.PackageName)
			if err != nil {
				return nil, err
			}
			state.Definitions = append(state.Definitions, def)
		}
	}

	state.Hints = append(state.Hints, fmt.Sprintf("Inspecting your deployment: %s",
		colors.Ctx(ctx).Highlight.Apply(fmt.Sprintf("kubectl -n %s get pods", target.namespace))))

	return &state, nil
}
