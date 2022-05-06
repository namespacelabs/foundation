// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	kubenode "namespacelabs.dev/foundation/std/runtime/kubernetes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
	"sigs.k8s.io/yaml"
)

var (
	ObserveInitContainerLogs = false

	runtimeCache struct {
		mu    sync.Mutex
		cache map[string]k8sRuntime
	}
)

func init() {
	runtimeCache.cache = map[string]k8sRuntime{}
}

func Register() {
	runtime.Register("kubernetes", func(ctx context.Context, ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) (runtime.Runtime, error) {
		return New(ctx, ws, devHost, env)
	})

	frontend.RegisterPrepareHook("namespacelabs.dev/foundation/std/runtime/kubernetes.ApplyServerExtensions", prepareApplyServerExtensions)
}

func prepareApplyServerExtensions(ctx context.Context, env ops.Environment, srv *schema.Server) (*frontend.PrepareProps, error) {
	var ensureServiceAccount bool

	if err := configure.VisitAllocs(srv.Allocation, kubeNode, &kubenode.ServerExtensionArgs{},
		func(instance *schema.Allocation_Instance, instantiate *schema.Instantiate, args *kubenode.ServerExtensionArgs) error {
			if args.EnsureServiceAccount {
				ensureServiceAccount = true
			}
			return nil
		}); err != nil {
		return nil, err
	}

	if !ensureServiceAccount {
		return nil, nil
	}

	serviceAccount := kubedef.MakeDeploymentId(srv)

	saDetails := &kubedef.ServiceAccountDetails{ServiceAccountName: serviceAccount}
	packedSaDetails, err := anypb.New(saDetails)
	if err != nil {
		return nil, err
	}

	packedExt, err := anypb.New(&kubedef.SpecExtension{
		EnsureServiceAccount: true,
		ServiceAccount:       serviceAccount,
	})
	if err != nil {
		return nil, err
	}

	return &frontend.PrepareProps{
		ProvisionInput: []*anypb.Any{packedSaDetails},
		Extension: []*schema.DefExtension{{
			For:  srv.PackageName,
			Impl: packedExt,
		}},
	}, nil
}

func New(ctx context.Context, ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) (k8sRuntime, error) {
	cfg, err := client.ComputeHostEnv(devHost, env)
	if err != nil {
		return k8sRuntime{}, err
	}

	keyBytes, err := json.Marshal(struct {
		C *client.HostEnv
		E *schema.Environment
	}{cfg, env})
	if err != nil {
		return k8sRuntime{}, fnerrors.InternalError("failed to serialize config/env key: %w", err)
	}

	key := string(keyBytes)

	runtimeCache.mu.Lock()
	defer runtimeCache.mu.Unlock()

	if _, ok := runtimeCache.cache[key]; !ok {
		cli, err := client.NewClientFromHostEnv(cfg)
		if err != nil {
			return k8sRuntime{}, err
		}

		runtimeCache.cache[key] = k8sRuntime{
			cli,
			boundEnv{ws, env, cfg},
			compute.InternalGetFuture[*kubedef.SystemInfo](ctx, &fetchSystemInfo{
				cli:     cli,
				cfg:     cfg,
				devHost: devHost,
				env:     env,
			}),
		}
	}

	rt := runtimeCache.cache[key]

	return rt, nil
}

type k8sRuntime struct {
	cli *k8s.Clientset
	boundEnv
	systemInfo *compute.Future[any] // systemInfo
}

var _ runtime.Runtime = k8sRuntime{}

func (r k8sRuntime) PrepareProvision(ctx context.Context) (*rtypes.ProvisionProps, error) {
	packedHostEnv, err := anypb.New(&kubetool.KubernetesEnv{Namespace: r.ns()})
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
		Name:        r.ns(),
		Body:        applycorev1.Namespace(r.ns()),
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

type ConditionWaiter interface {
	Prepare(context.Context, *k8s.Clientset) error
	Poll(context.Context, *k8s.Clientset) (bool, error)
}

func (r k8sRuntime) Wait(ctx context.Context, action *tasks.ActionEvent, waiter ConditionWaiter) error {
	return waitForCondition(ctx, r.cli, action, waiter)
}

func waitForCondition(ctx context.Context, cli *k8s.Clientset, action *tasks.ActionEvent, waiter ConditionWaiter) error {
	return action.Run(ctx, func(ctx context.Context) error {
		if err := waiter.Prepare(ctx, cli); err != nil {
			return err
		}

		return client.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(ctx context.Context) (bool, error) {
			return waiter.Poll(ctx, cli)
		})
	})
}

func (r k8sRuntime) DeployedConfigImageID(ctx context.Context, server *schema.Server) (oci.ImageID, error) {
	d, err := r.cli.AppsV1().Deployments(r.ns()).Get(ctx, kubedef.MakeDeploymentId(server), metav1.GetOptions{})
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

func (r k8sRuntime) PrepareCluster(ctx context.Context) (runtime.DeploymentState, error) {
	var state deploymentState

	ingressDefs, err := ingress.EnsureStack(ctx)
	if err != nil {
		return nil, err
	}

	state.definitions = ingressDefs

	return state, nil
}

func (r k8sRuntime) PlanDeployment(ctx context.Context, d runtime.Deployment) (runtime.DeploymentState, error) {
	var state deploymentState

	for _, server := range d.Servers {
		var singleState serverRunState
		var deployOpts deployOpts

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

	state.hints = append(state.hints, fmt.Sprintf("Track your deployment with %s.", colors.Bold(fmt.Sprintf("kubectl -n %s get pods", r.ns()))))

	return state, nil
}

func (r k8sRuntime) PlanIngress(ctx context.Context, stack *schema.Stack, allFragments []*schema.IngressFragment) (runtime.DeploymentState, error) {
	var state deploymentState

	certSecretMap, secrets := ingress.MakeCertificateSecrets(r.ns(), allFragments)

	for _, apply := range secrets {
		// XXX we could actually collect which servers refer what certs, to use as scope.
		def, err := apply.ToDefinition()
		if err != nil {
			return nil, err
		}
		state.definitions = append(state.definitions, def)
	}

	// XXX ensure that any single domain is only used by a single ingress.
	var managed []ingress.MapAddress
	for _, srv := range stack.Entry {
		var frags []*schema.IngressFragment
		for _, fr := range allFragments {
			if srv.GetPackageName().Equals(fr.Owner) {
				frags = append(frags, fr)
			}
		}

		if len(frags) == 0 {
			continue
		}

		defs, m, err := ingress.Ensure(ctx, r.ns(), r.env, srv.Server, frags, certSecretMap)
		if err != nil {
			return nil, err
		}

		for _, apply := range defs {
			def, err := apply.ToDefinition(srv.GetPackageName())
			if err != nil {
				return nil, err
			}
			state.definitions = append(state.definitions, def)
		}

		managed = append(managed, m...)
	}

	// XXX this could be reduced in effort (e.g. batched).
	for _, frag := range managed {
		impl, err := anypb.New(&ingress.OpMapAddress{
			Fdqn:        frag.FQDN,
			IngressNs:   frag.Ingress.Namespace,
			IngressName: frag.Ingress.Name,
		})
		if err != nil {
			return nil, err
		}

		state.definitions = append(state.definitions, &schema.Definition{
			Description: fmt.Sprintf("Update %s's address", frag.FQDN),
			Impl:        impl,
		})
	}

	return state, nil
}

func (r k8sRuntime) PlanShutdown(ctx context.Context, foci []provision.Server, stack []provision.Server) ([]*schema.Definition, error) {
	var definitions []*schema.Definition

	if del, err := ingress.Delete(r.ns(), stack); err != nil {
		return nil, err
	} else {
		definitions = append(definitions, del...)
	}

	for _, t := range stack {
		var ops []defs.MakeDefinition

		ops = append(ops, kubedef.DeleteList{
			Description: "Services",
			Resource:    "services",
			Namespace:   r.ns(),
			Selector:    kubedef.SelectById(t.Proto()),
		})

		if t.IsStateful() {
			ops = append(ops, kubedef.Delete{
				Description: "StatefulSet",
				Resource:    "statefulsets",
				Namespace:   r.ns(),
				Name:        kubedef.MakeDeploymentId(t.Proto()),
			})
		} else {
			ops = append(ops, kubedef.Delete{
				Description: "Deployment",
				Resource:    "deployments",
				Namespace:   r.ns(),
				Name:        kubedef.MakeDeploymentId(t.Proto()),
			})
		}

		for _, op := range ops {
			if def, err := op.ToDefinition(t.PackageName()); err != nil {
				return nil, err
			} else {
				definitions = append(definitions, def)
			}
		}
	}

	return definitions, nil
}

func (r k8sRuntime) StreamLogsTo(ctx context.Context, w io.Writer, server *schema.Server, opts runtime.StreamLogsOpts) error {
	return r.fetchLogs(ctx, r.cli, w, server, opts)
}

func (r k8sRuntime) FetchLogsTo(ctx context.Context, w io.Writer, reference runtime.ContainerReference, opts runtime.FetchLogsOpts) error {
	opaque, ok := reference.(containerPodReference)
	if !ok {
		return fnerrors.InternalError("invalid reference")
	}

	return fetchPodLogs(ctx, r.cli, w, opaque.Namespace, opaque.Name, opaque.Container, runtime.StreamLogsOpts{
		TailLines:        opts.TailLines,
		FetchLastFailure: opts.FetchLastFailure,
	})
}

func (r k8sRuntime) FetchDiagnostics(ctx context.Context, reference runtime.ContainerReference) (runtime.Diagnostics, error) {
	opaque, ok := reference.(containerPodReference)
	if !ok {
		return runtime.Diagnostics{}, fnerrors.InternalError("invalid reference")
	}

	pod, err := r.cli.CoreV1().Pods(opaque.Namespace).Get(ctx, opaque.Name, metav1.GetOptions{})
	if err != nil {
		return runtime.Diagnostics{}, err
	}

	for _, init := range pod.Status.InitContainerStatuses {
		if init.Name == opaque.Container {
			return statusToDiagnostic(init), nil
		}
	}

	for _, ctr := range pod.Status.ContainerStatuses {
		if ctr.Name == opaque.Container {
			return statusToDiagnostic(ctr), nil
		}
	}

	return runtime.Diagnostics{}, fnerrors.UserError(nil, "%s/%s: no such container %q", opaque.Namespace, opaque.Name, opaque.Container)
}

func statusToDiagnostic(status v1.ContainerStatus) runtime.Diagnostics {
	var diag runtime.Diagnostics

	diag.RestartCount = status.RestartCount

	switch {
	case status.State.Running != nil:
		diag.Running = true
		diag.Started = status.State.Running.StartedAt.Time
	case status.State.Waiting != nil:
		diag.Waiting = true
		diag.WaitingReason = status.State.Waiting.Reason
		diag.Crashed = status.State.Waiting.Reason == "CrashLoopBackOff"
	case status.State.Terminated != nil:
		diag.Terminated = true
		diag.TerminatedReason = status.State.Terminated.Reason
		diag.ExitCode = status.State.Terminated.ExitCode
	}

	return diag
}

func (r k8sRuntime) StartTerminal(ctx context.Context, server *schema.Server, rio runtime.TerminalIO, command string, rest ...string) error {
	cmd := append([]string{command}, rest...)

	return r.startTerminal(ctx, r.cli, server, rio, cmd)
}

func (r k8sRuntime) ForwardPort(ctx context.Context, server *schema.Server, endpoint *schema.Endpoint, localAddrs []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
	if endpoint.GetPort().GetContainerPort() <= 0 {
		return nil, fnerrors.UserError(server, "%s: no port to forward to", endpoint.GetServiceName())
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)
	p := newPodResolver(r.cli, r.boundEnv.ns(), map[string]string{
		kubedef.K8sServerId: server.Id,
	})

	p.Start(ctxWithCancel)

	go func() {
		if err := r.boundEnv.startAndBlockPortFwd(ctxWithCancel, fwdArgs{
			Namespace:     r.boundEnv.ns(),
			Identifier:    server.PackageName,
			LocalAddrs:    localAddrs,
			LocalPort:     0,
			ContainerPort: int(endpoint.GetPort().ContainerPort),

			Watch: func(ctx context.Context, f func(*v1.Pod, int64, error)) func() {
				return p.Watch(f)
			},
			ReportPorts: callback,
		}); err != nil {
			fmt.Fprintf(console.Errors(ctx), "port forwarding for %s (%d) failed: %v\n", server.PackageName, endpoint.GetPort().ContainerPort, err)
		}
	}()

	return closerCallback(cancel), nil
}

func (r k8sRuntime) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f runtime.PortForwardedFunc) (io.Closer, error) {
	svc := nginx.IngressLoadBalancerService()
	// XXX watch?
	resolved, err := r.cli.CoreV1().Services(svc.Namespace).Get(ctx, svc.ServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	pod, err := resolvePodByLabels(ctx, r.cli, io.Discard, svc.Namespace, resolved.Spec.Selector)
	if err != nil {
		return nil, err
	}

	ctxWithCancel, cancel := context.WithCancel(ctx)

	go func() {
		if err := r.boundEnv.startAndBlockPortFwd(ctxWithCancel, fwdArgs{
			Namespace:     svc.Namespace,
			Identifier:    "ingress",
			LocalAddrs:    localAddrs,
			LocalPort:     localPort,
			ContainerPort: svc.ContainerPort,

			Watch: func(_ context.Context, f func(*v1.Pod, int64, error)) func() {
				f(&pod, 1, nil)
				return func() {}
			},
			ReportPorts: func(p runtime.ForwardedPort) {
				f(runtime.ForwardedPortEvent{
					Added: []runtime.ForwardedPort{{
						LocalPort:     p.LocalPort,
						ContainerPort: p.ContainerPort,
					}},
					Endpoint: &schema.Endpoint{
						ServiceName: runtime.IngressServiceName,
						ServiceMetadata: []*schema.ServiceMetadata{{
							Protocol: "http",
							Kind:     runtime.IngressServiceKind,
						}},
					},
				})
			},
		}); err != nil {
			fmt.Fprintf(console.Errors(ctx), "ingress forwarding failed: %v\n", err)
		}
	}()

	return closerCallback(cancel), nil
}

type closerCallback func()

func (c closerCallback) Close() error {
	c()
	return nil
}

func (r k8sRuntime) Observe(ctx context.Context, srv *schema.Server, opts runtime.ObserveOpts, onInstance func(runtime.ObserveEvent) error) error {
	// XXX use a watch
	announced := map[string]runtime.ContainerReference{}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// No cancelation, moving along.
		}

		pods, err := r.cli.CoreV1().Pods(r.ns()).List(ctx, metav1.ListOptions{
			LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(srv)),
		})
		if err != nil {
			return err
		}

		type Key struct {
			Instance  runtime.ContainerReference
			CreatedAt time.Time // used for sorting
		}
		keys := []Key{}
		newM := map[string]struct{}{}
		labels := map[string]string{}
		for _, pod := range pods.Items {
			if pod.Status.Phase == v1.PodRunning {
				instance := makePodRef(r.ns(), pod.Name, serverCtrName(srv))
				keys = append(keys, Key{
					Instance:  instance,
					CreatedAt: pod.CreationTimestamp.Time,
				})
				newM[instance.UniqueID()] = struct{}{}
				labels[instance.UniqueID()] = pod.Name

				if ObserveInitContainerLogs {
					for _, container := range pod.Spec.InitContainers {
						instance := makePodRef(r.ns(), pod.Name, container.Name)
						keys = append(keys, Key{Instance: instance, CreatedAt: pod.CreationTimestamp.Time})
						newM[instance.UniqueID()] = struct{}{}
						labels[instance.UniqueID()] = fmt.Sprintf("%s:%s", pod.Name, container.Name)
					}
				}
			}
		}
		sort.SliceStable(keys, func(i int, j int) bool {
			return keys[i].CreatedAt.Before(keys[j].CreatedAt)
		})

		for k, ref := range announced {
			if _, ok := newM[k]; ok {
				delete(newM, k)
			} else {
				if err := onInstance(runtime.ObserveEvent{ContainerReference: ref, Removed: true}); err != nil {
					return err
				}
			}
		}

		for _, key := range keys {
			instance := key.Instance
			if _, ok := newM[instance.UniqueID()]; !ok {
				continue
			}
			human := labels[instance.UniqueID()]
			if human == "" {
				human = instance.HumanReference()
			}

			if err := onInstance(runtime.ObserveEvent{ContainerReference: instance, HumanReadableID: human, Added: true}); err != nil {
				return err
			}
			announced[instance.UniqueID()] = instance
		}

		if opts.OneShot {
			return nil
		}

		time.Sleep(1 * time.Second)
	}
}

func (r k8sRuntime) DeleteRecursively(ctx context.Context) error {
	return tasks.Action("kubernetes.namespace.delete").Arg("namespace", r.ns()).Run(ctx, func(ctx context.Context) error {
		var grace int64 = 0
		return r.cli.CoreV1().Namespaces().Delete(ctx, r.ns(), metav1.DeleteOptions{
			GracePeriodSeconds: &grace,
		})
	})
}
