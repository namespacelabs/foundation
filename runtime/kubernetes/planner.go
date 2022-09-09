// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"bytes"
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
	"sigs.k8s.io/yaml"
)

type planner struct {
	target clusterTarget
}

var _ runtime.Namespace = planner{}
var _ runtime.Planner = planner{}

func (r planner) UniqueID() string {
	return fmt.Sprintf("kubernetes:%s", r.target.namespace)
}

func (r planner) Planner() runtime.Planner {
	return r
}

func (r planner) PlanDeployment(ctx context.Context, d runtime.Deployment) (runtime.DeploymentState, error) {
	return planDeployment(ctx, r.target, d)
}

func (r planner) PlanIngress(ctx context.Context, stack *schema.Stack, allFragments []*schema.IngressFragment) (runtime.DeploymentState, error) {
	return planIngress(ctx, r.target, stack, allFragments)
}

func (r planner) Namespace() runtime.Namespace {
	return r
	// id := &runtime.NamespaceId{
	// 	HumanReference: fmt.Sprintf("kubernetes:%s", r.target.namespace),
	// }

	// hash := sha256.Sum256([]byte(id.HumanReference))
	// id.UniqueId = hex.EncodeToString(hash[:])

	// return id, nil
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
