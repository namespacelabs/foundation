// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
	"sigs.k8s.io/yaml"
)

var (
	ObserveInitContainerLogs = false

	runtimeCache struct {
		mu    sync.Mutex
		cache map[string]K8sRuntime
	}
)

func init() {
	runtimeCache.cache = map[string]K8sRuntime{}
}

func Register() {
	runtime.Register("kubernetes", func(ctx context.Context, ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) (runtime.Runtime, error) {
		return New(ctx, ws, devHost, env)
	})

	frontend.RegisterPrepareHook("namespacelabs.dev/foundation/std/runtime/kubernetes.ApplyServerExtensions", prepareApplyServerExtensions)
}

func NewFromConfig(ctx context.Context, config *HostConfig) (K8sRuntime, error) {
	keyBytes, err := json.Marshal(struct {
		C *client.HostEnv
		E *schema.Environment
	}{config.hostEnv, config.env})
	if err != nil {
		return K8sRuntime{}, fnerrors.InternalError("failed to serialize config/env key: %w", err)
	}

	key := string(keyBytes)

	runtimeCache.mu.Lock()
	defer runtimeCache.mu.Unlock()

	if _, ok := runtimeCache.cache[key]; !ok {
		cli, err := client.NewClientFromHostEnv(ctx, config.hostEnv)
		if err != nil {
			return K8sRuntime{}, err
		}

		runtimeCache.cache[key] = K8sRuntime{
			cli,
			boundEnv{config.ws, config.env, config.hostEnv, moduleNamespace(config.ws, config.env)},
		}
	}

	rt := runtimeCache.cache[key]

	return rt, nil
}

func New(ctx context.Context, ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) (K8sRuntime, error) {
	hostEnv, err := client.ComputeHostEnv(devHost, env)
	if err != nil {
		return K8sRuntime{}, err
	}
	hostConfig := &HostConfig{ws: ws, devHost: devHost, env: env, hostEnv: hostEnv, registry: nil}
	return NewFromConfig(ctx, hostConfig)
}

type K8sRuntime struct {
	cli *k8s.Clientset
	boundEnv
}

var _ runtime.Runtime = K8sRuntime{}

func (r K8sRuntime) PrepareProvision(ctx context.Context) (*rtypes.ProvisionProps, error) {
	packedHostEnv, err := anypb.New(&kubetool.KubernetesEnv{Namespace: r.moduleNamespace})
	if err != nil {
		return nil, err
	}

	systemInfo, err := r.SystemInfo(ctx)
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
		Description: "Namespace",
		Resource:    "namespaces",
		Name:        r.moduleNamespace,
		Body: applycorev1.Namespace(r.moduleNamespace).
			WithLabels(kubedef.MakeLabels(r.env, nil)).
			WithAnnotations(kubedef.MakeAnnotations(r.env, nil)),
	}).ToDefinition()
	if err != nil {
		return nil, err
	}

	// Pass the computed namespace to the provisioning tool.
	return &rtypes.ProvisionProps{
		ProvisionInput: []*anypb.Any{packedHostEnv, packedSystemInfo},
		Definition:     []*schema.Definition{def},
	}, nil
}

type serverRunState struct {
	declarations []kubedef.Apply
}

type deploymentState struct {
	definitions []*schema.Definition
	hints       []string // Optional messages to pass to the user.
}

func (r deploymentState) Definitions() []*schema.Definition {
	return r.definitions
}

func (r deploymentState) Hints() []string {
	return r.hints
}

func (r K8sRuntime) DeployedConfigImageID(ctx context.Context, server *schema.Server) (oci.ImageID, error) {
	// XXX need a StatefulSet variant.
	d, err := r.cli.AppsV1().Deployments(serverNamespace(r.boundEnv, server)).Get(ctx, kubedef.MakeDeploymentId(server), metav1.GetOptions{})
	if err != nil {
		// XXX better error messages.
		return oci.ImageID{}, err
	}

	cfgimage, ok := d.Annotations[kubedef.K8sConfigImage]
	if !ok {
		return oci.ImageID{}, fnerrors.BadInputError("%s: %q is missing as an annotation in %q",
			server.GetPackageName(), kubedef.K8sConfigImage, kubedef.MakeDeploymentId(server))
	}

	return oci.ParseImageID(cfgimage)
}

func (r K8sRuntime) PrepareCluster(ctx context.Context) (runtime.DeploymentState, error) {
	var state deploymentState

	ingressDefs, err := ingress.EnsureStack(ctx)
	if err != nil {
		return nil, err
	}

	state.definitions = ingressDefs

	return state, nil
}

func (r K8sRuntime) PlanDeployment(ctx context.Context, d runtime.Deployment) (runtime.DeploymentState, error) {
	var state deploymentState
	deployOpts := deployOpts{
		focus: d.Focus,
	}

	// Collect all required servers before planning deployment as they are referenced in annotations.
	for _, server := range d.Servers {
		deployOpts.stackIds = append(deployOpts.stackIds, server.Server.Proto().Id)
	}

	for _, server := range d.Servers {
		var singleState serverRunState

		var serverInternalEndpoints []*schema.InternalEndpoint
		for _, ie := range d.Stack.InternalEndpoint {
			if server.Server.PackageName().Equals(ie.ServerOwner) {
				serverInternalEndpoints = append(serverInternalEndpoints, ie)
			}
		}

		if err := r.prepareServerDeployment(ctx, server, serverInternalEndpoints, deployOpts, &singleState); err != nil {
			return nil, err
		}

		// XXX verify we've consumed all endpoints.
		for _, endpoint := range d.Stack.EndpointsBy(server.Server.PackageName()) {
			if err := r.deployEndpoint(ctx, server, endpoint, &singleState); err != nil {
				return nil, err
			}
		}

		if at := tasks.Attachments(ctx); at.IsStoring() {
			output := &bytes.Buffer{}
			for k, decl := range singleState.declarations {
				if k > 0 {
					fmt.Fprintln(output, "---")
				}

				b, err := yaml.Marshal(decl.Body)
				if err == nil {
					fmt.Fprintf(output, "%s\n", b)
					// XXX ignoring errors
				}
			}

			at.Attach(tasks.Output(fmt.Sprintf("%s.k8s-decl.yaml", server.Server.PackageName()), "application/yaml"), output.Bytes())
		}

		for _, apply := range singleState.declarations {
			def, err := apply.ToDefinition(server.Server.PackageName())
			if err != nil {
				return nil, err
			}
			state.definitions = append(state.definitions, def)
		}
	}

	state.hints = append(state.hints, fmt.Sprintf("Inspecting your deployment: %s", colors.Bold(fmt.Sprintf("kubectl -n %s get pods", r.moduleNamespace))))

	return state, nil
}

func (r K8sRuntime) StartTerminal(ctx context.Context, server *schema.Server, rio runtime.TerminalIO, command string, rest ...string) error {
	cmd := append([]string{command}, rest...)

	return r.startTerminal(ctx, r.cli, server, rio, cmd)
}

func (r K8sRuntime) AttachTerminal(ctx context.Context, reference runtime.ContainerReference, rio runtime.TerminalIO) error {
	opaque, ok := reference.(kubedef.ContainerPodReference)
	if !ok {
		return fnerrors.InternalError("invalid reference")
	}

	return r.attachTerminal(ctx, r.cli, opaque, rio)
}

func (r K8sRuntime) Wait(ctx context.Context, action *tasks.ActionEvent, waiter kubeobserver.ConditionWaiter) error {
	return kubeobserver.WaitForCondition(ctx, r.cli, action, waiter)
}
