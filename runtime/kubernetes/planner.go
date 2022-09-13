// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"bytes"
	"context"
	"fmt"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
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

func (r Planner) PlanDeployment(ctx context.Context, d runtime.Deployment) (runtime.DeploymentState, error) {
	return planDeployment(ctx, r.target, d)
}

func (r Planner) PlanIngress(ctx context.Context, stack *schema.Stack, allFragments []*schema.IngressFragment) (runtime.DeploymentState, error) {
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

func (r Planner) ComputeBaseNaming(*schema.Naming) (*schema.ComputedNaming, error) {
	// The default kubernetes integration has no assumptions regarding how ingress names are allocated.
	return nil, nil
}

func (r Planner) TargetPlatforms(ctx context.Context) ([]specs.Platform, error) {
	if !UseNodePlatformsForProduction && r.target.env.Purpose == schema.Environment_PRODUCTION {
		return parsePlatforms(ProductionPlatforms)
	}

	systemInfo, err := r.fetchSystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	return parsePlatforms(systemInfo.NodePlatform)
}

func planDeployment(ctx context.Context, r clusterTarget, d runtime.Deployment) (runtime.DeploymentState, error) {
	var state deploymentState
	deployOpts := deployOpts{
		focus:   d.Focus,
		secrets: d.Secrets,
	}

	// Collect all required servers before planning deployment as they are referenced in annotations.
	for _, server := range d.Servers {
		deployOpts.stackIds = append(deployOpts.stackIds, server.Server.Id)
	}

	for _, server := range d.Servers {
		var singleState serverRunState

		var serverInternalEndpoints []*schema.InternalEndpoint
		for _, ie := range d.Stack.InternalEndpoint {
			if server.Server.PackageName == ie.ServerOwner {
				serverInternalEndpoints = append(serverInternalEndpoints, ie)
			}
		}

		if err := prepareServerDeployment(ctx, r, server, serverInternalEndpoints, deployOpts, &singleState); err != nil {
			return nil, err
		}

		// XXX verify we've consumed all endpoints.
		for _, endpoint := range d.Stack.EndpointsBy(schema.PackageName(server.Server.PackageName)) {
			if err := deployEndpoint(ctx, r, server.Server, endpoint, &singleState); err != nil {
				return nil, err
			}
		}

		if at := tasks.Attachments(ctx); at.IsStoring() {
			output := &bytes.Buffer{}
			for k, decl := range singleState.operations {
				if k > 0 {
					fmt.Fprintln(output, "---")
				}

				b, err := yaml.Marshal(decl.Resource)
				if err == nil {
					fmt.Fprintf(output, "%s\n", b)
					// XXX ignoring errors
				}
			}

			at.Attach(tasks.Output(fmt.Sprintf("%s.k8s-decl.yaml", server.Server.PackageName), "application/yaml"), output.Bytes())
		}

		for _, apply := range singleState.operations {
			def, err := apply.ToDefinition(schema.PackageName(server.Server.PackageName))
			if err != nil {
				return nil, err
			}
			state.definitions = append(state.definitions, def)
		}
	}

	if !r.env.Ephemeral {
		cleanup, err := anypb.New(&kubedef.OpCleanupRuntimeConfig{
			Namespace: r.namespace,
			CheckPods: deployAsPods(r.env),
		})
		if err != nil {
			return nil, fnerrors.InternalError("failed to serialize cleanup: %w", err)
		}

		state.definitions = append(state.definitions, &schema.SerializedInvocation{
			Description: "Kubernetes: cleanup unused resources",
			Impl:        cleanup,
		})
	}

	state.hints = append(state.hints, fmt.Sprintf("Inspecting your deployment: %s",
		colors.Ctx(ctx).Highlight.Apply(fmt.Sprintf("kubectl -n %s get pods", r.namespace))))

	return state, nil
}
