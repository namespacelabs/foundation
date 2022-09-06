// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"bytes"
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
	"sigs.k8s.io/yaml"
)

var (
	ObserveInitContainerLogs = false
)

func Register() {
	runtime.Register("kubernetes", func(ctx context.Context, env planning.Context) (runtime.DeferredRuntime, error) {
		hostConfig, err := client.ComputeHostConfig(env.Configuration())
		if err != nil {
			return nil, err
		}

		fmt.Fprintf(console.Debug(ctx), "kubernetes: selected %+v for %q\n", hostConfig.HostEnv, env.Environment().Name)

		p, err := client.MakeDeferredRuntime(ctx, hostConfig)
		if err != nil {
			return nil, err
		}

		if p != nil {
			return p, nil
		}

		return deferredRuntime{}, nil
	})

	frontend.RegisterPrepareHook("namespacelabs.dev/foundation/std/runtime/kubernetes.ApplyServerExtensions", prepareApplyServerExtensions)
}

type deferredRuntime struct{}

func (d deferredRuntime) New(ctx context.Context, env planning.Context) (runtime.Runtime, error) {
	unbound, err := New(ctx, env.Configuration())
	if err != nil {
		return nil, err
	}

	return unbound.Bind(env), nil
}

func MakeNamespace(env *schema.Environment, ns string) *applycorev1.NamespaceApplyConfiguration {
	return applycorev1.Namespace(ns).
		WithLabels(kubedef.MakeLabels(env, nil)).
		WithAnnotations(kubedef.MakeAnnotations(env, nil))
}

func (r K8sRuntime) PrepareProvision(ctx context.Context, _ planning.Context) (*rtypes.ProvisionProps, error) {
	systemInfo, err := r.SystemInfo(ctx)
	if err != nil {
		return nil, err
	}

	return PrepareProvisionWith(r.env, r.moduleNamespace, systemInfo)
}

func PrepareProvisionWith(env *schema.Environment, moduleNamespace string, systemInfo *kubedef.SystemInfo) (*rtypes.ProvisionProps, error) {
	packedHostEnv, err := anypb.New(&kubetool.KubernetesEnv{Namespace: moduleNamespace})
	if err != nil {
		return nil, err
	}

	packedSystemInfo, err := anypb.New(systemInfo)
	if err != nil {
		return nil, err
	}

	// Ensure the namespace exist, before we go and apply definitions to it. Also, deployServer
	// assumes that a namespace already exists.
	def, err := (kubedef.Apply{
		Description: fmt.Sprintf("Namespace for %q", env.Name),
		Resource:    MakeNamespace(env, moduleNamespace),
	}).ToDefinition()
	if err != nil {
		return nil, err
	}

	// Pass the computed namespace to the provisioning tool.
	return &rtypes.ProvisionProps{
		ProvisionInput: []*anypb.Any{packedHostEnv, packedSystemInfo},
		Invocation:     []*schema.SerializedInvocation{def},
	}, nil
}

type serverRunState struct {
	operations []kubedef.Apply
}

type deploymentState struct {
	definitions []*schema.SerializedInvocation
	hints       []string // Optional messages to pass to the user.
}

func (r deploymentState) Definitions() []*schema.SerializedInvocation {
	return r.definitions
}

func (r deploymentState) Hints() []string {
	return r.hints
}

func (r K8sRuntime) DeployedConfigImageID(ctx context.Context, server *schema.Server) (oci.ImageID, error) {
	return tasks.Return(ctx, tasks.Action("kubernetes.resolve-config-image-id").Scope(schema.PackageName(server.PackageName)),
		func(ctx context.Context) (oci.ImageID, error) {
			// XXX need a StatefulSet variant.
			d, err := r.cli.AppsV1().Deployments(serverNamespace(r, server)).Get(ctx, kubedef.MakeDeploymentId(server), metav1.GetOptions{})
			if err != nil {
				// XXX better error messages.
				return oci.ImageID{}, err
			}

			cfgimage, ok := d.Annotations[kubedef.K8sConfigImage]
			if !ok {
				return oci.ImageID{}, fnerrors.BadInputError("%s: %q is missing as an annotation in %q",
					server.GetPackageName(), kubedef.K8sConfigImage, kubedef.MakeDeploymentId(server))
			}

			imgid, err := oci.ParseImageID(cfgimage)
			if err != nil {
				return imgid, err
			}

			tasks.Attachments(ctx).AddResult("ref", imgid.ImageRef())

			return imgid, nil
		})
}

func (r K8sRuntime) PlanDeployment(ctx context.Context, d runtime.Deployment) (runtime.DeploymentState, error) {
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

		if err := r.prepareServerDeployment(ctx, server, serverInternalEndpoints, deployOpts, &singleState); err != nil {
			return nil, err
		}

		// XXX verify we've consumed all endpoints.
		for _, endpoint := range d.Stack.EndpointsBy(schema.PackageName(server.Server.PackageName)) {
			if err := r.deployEndpoint(ctx, server.Server, endpoint, &singleState); err != nil {
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
			Namespace: r.moduleNamespace,
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

	state.hints = append(state.hints, fmt.Sprintf("Inspecting your deployment: %s", colors.Ctx(ctx).Highlight.Apply(fmt.Sprintf("kubectl -n %s get pods", r.moduleNamespace))))

	return state, nil
}

func (r K8sRuntime) ComputeBaseNaming(context.Context, *schema.Naming) (*schema.ComputedNaming, error) {
	// The default kubernetes integration has no assumptions regarding how ingress names are allocated.
	return nil, nil
}

func (r K8sRuntime) StartTerminal(ctx context.Context, server *schema.Server, rio runtime.TerminalIO, command string, rest ...string) error {
	cmd := append([]string{command}, rest...)

	return r.startTerminal(ctx, r.cli, server, rio, cmd)
}

func (r K8sRuntime) AttachTerminal(ctx context.Context, reference *runtime.ContainerReference, rio runtime.TerminalIO) error {
	cpr := &kubedef.ContainerPodReference{}
	if err := reference.Opaque.UnmarshalTo(cpr); err != nil {
		return fnerrors.InternalError("invalid reference: %w", err)
	}

	return r.attachTerminal(ctx, r.cli, cpr, rio)
}
