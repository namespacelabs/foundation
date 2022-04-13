// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"google.golang.org/protobuf/types/known/anypb"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/portforward"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
	"sigs.k8s.io/yaml"
)

var (
	ObserveInitContainerLogs = false
)

func Register() {
	runtime.Register("kubernetes", func(ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) (runtime.Runtime, error) {
		return New(ws, devHost, env)
	})
}

func New(ws *schema.Workspace, devHost *schema.DevHost, env *schema.Environment) (k8sRuntime, error) {
	cfg, err := client.ComputeHostEnv(devHost, env)
	if err != nil {
		return k8sRuntime{}, err
	}

	return k8sRuntime{boundEnv{ws, env, cfg}}, nil
}

type k8sRuntime struct {
	boundEnv
}

var _ runtime.Runtime = k8sRuntime{}

func (r k8sRuntime) PrepareProvision(ctx context.Context) (*rtypes.ProvisionProps, error) {
	packedHostEnv, err := anypb.New(&kubetool.KubernetesEnv{Namespace: r.ns()})
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
		ProvisionInput: []*anypb.Any{packedHostEnv},
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

func (r k8sRuntime) Wait(ctx context.Context, action *tasks.ActionEvent, f func(context.Context, *k8s.Clientset) (bool, error)) error {
	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return err
	}

	return waitForCondition(ctx, cli, action, f)
}

func waitForCondition(ctx context.Context, cli *k8s.Clientset, action *tasks.ActionEvent, f func(context.Context, *k8s.Clientset) (bool, error)) error {
	return action.Run(ctx, func(ctx context.Context) error {
		return wait.PollImmediateWithContext(ctx, 500*time.Millisecond, 5*time.Minute, func(c context.Context) (done bool, err error) {
			return f(c, cli)
		})
	})
}

func (r k8sRuntime) DeployedConfigImageID(ctx context.Context, server *schema.Server) (oci.ImageID, error) {
	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return oci.ImageID{}, err
	}

	d, err := cli.AppsV1().Deployments(r.ns()).Get(ctx, kubedef.MakeDeploymentId(server), metav1.GetOptions{})
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
	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return err
	}

	return r.fetchLogs(ctx, cli, w, server, opts)
}

func (r k8sRuntime) StartTerminal(ctx context.Context, server *schema.Server, rio runtime.TerminalIO, command string, rest ...string) error {
	cmd := append([]string{command}, rest...)

	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return err
	}

	return r.startTerminal(ctx, cli, server, rio, cmd)
}

func (r k8sRuntime) ForwardPort(ctx context.Context, server *schema.Server, endpoint *schema.Endpoint, localAddrs []string, callback runtime.SinglePortForwardedFunc) (io.Closer, error) {
	if endpoint.GetPort().GetContainerPort() <= 0 {
		return nil, fnerrors.UserError(server, "%s: no port to forward to", endpoint.GetServiceName())
	}

	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return nil, err
	}

	pod, err := resolvePodByLabels(ctx, cli, io.Discard, r.boundEnv.ns(), map[string]string{
		kubedef.K8sServerId: server.Id,
	})
	if err != nil {
		return nil, err
	}

	// XXX be smarter about port allocation.
	containerPorts := []string{fmt.Sprintf(":%d", endpoint.GetPort().ContainerPort)}

	stopCh := make(chan struct{})

	compute.On(ctx).DetachWith(compute.Detach{
		Action:     tasks.Action("port.forward").Indefinite(),
		BestEffort: true,
		Do: func(ctx context.Context) error {
			return r.boundEnv.startAndBlockPortFwd(ctx, r.boundEnv.ns(), pod.Name, localAddrs, containerPorts, stopCh, func(forwarded []portforward.ForwardedPort) {
				for _, p := range forwarded {
					callback(runtime.ForwardedPort{
						LocalPort:     uint(p.Local),
						ContainerPort: uint(endpoint.Port.GetContainerPort()),
					})
				}
			})
		},
	})

	return channelCloser(stopCh), nil
}

func (r k8sRuntime) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, f runtime.PortForwardedFunc) (io.Closer, error) {
	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return nil, err
	}

	svc := nginx.IngressLoadBalancerService()
	// XXX watch?
	resolved, err := cli.CoreV1().Services(svc.Namespace).Get(ctx, svc.ServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	pod, err := resolvePodByLabels(ctx, cli, io.Discard, svc.Namespace, resolved.Spec.Selector)
	if err != nil {
		return nil, err
	}

	stopCh := make(chan struct{})

	compute.On(ctx).DetachWith(compute.Detach{
		Action:     tasks.Action("port.forward.ingress").Indefinite(),
		BestEffort: true,
		Do: func(ctx context.Context) error {
			return r.startAndBlockPortFwd(ctx, svc.Namespace, pod.Name, localAddrs, []string{fmt.Sprintf("%d:%d", localPort, svc.ContainerPort)}, stopCh, func(forwarded []portforward.ForwardedPort) {
				for _, p := range forwarded {
					f(runtime.ForwardedPortEvent{
						Added: []runtime.ForwardedPort{{
							LocalPort:     uint(p.Local),
							ContainerPort: uint(svc.ContainerPort),
						}},
						Endpoint: &schema.Endpoint{
							ServiceName: runtime.IngressServiceName,
							ServiceMetadata: []*schema.ServiceMetadata{{
								Protocol: "http",
								Kind:     runtime.IngressServiceKind,
							}},
						},
					})
				}
			})
		},
	})

	return channelCloser(stopCh), nil
}

type channelCloser chan struct{}

func (c channelCloser) Close() error {
	close(c)
	return nil
}

func (r k8sRuntime) Observe(ctx context.Context, srv *schema.Server, opts runtime.ObserveOpts, onInstance func(runtime.ObserveEvent) error) error {
	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return err
	}

	// XXX use a watch
	announced := map[string]struct{}{}

	for {
		// XXX check for cancelation.

		pods, err := cli.CoreV1().Pods(r.ns()).List(ctx, metav1.ListOptions{
			LabelSelector: kubedef.SerializeSelector(kubedef.SelectById(srv)),
		})
		if err != nil {
			return err
		}

		type Key struct {
			Instance  string
			CreatedAt time.Time // used for sorting
		}
		keys := []Key{}
		newM := map[string]struct{}{}
		labels := map[string]string{}
		for _, pod := range pods.Items {
			if pod.Status.Phase == corev1.PodRunning {
				id := pod.Name
				keys = append(keys, Key{Instance: id, CreatedAt: pod.CreationTimestamp.Time})
				newM[id] = struct{}{}
				labels[id] = pod.Name

				if ObserveInitContainerLogs {
					for _, container := range pod.Spec.InitContainers {
						id := fmt.Sprintf("%s:%s", pod.Name, container.Name)
						keys = append(keys, Key{Instance: id, CreatedAt: pod.CreationTimestamp.Time})
						newM[id] = struct{}{}
						labels[id] = fmt.Sprintf("%s:%s", pod.Name, container.Name)
					}
				}
			}
		}
		sort.SliceStable(keys, func(i int, j int) bool {
			return keys[i].CreatedAt.Before(keys[j].CreatedAt)
		})

		for k := range announced {
			if _, ok := newM[k]; ok {
				delete(newM, k)
			} else {
				if err := onInstance(runtime.ObserveEvent{InstanceID: k, Removed: true}); err != nil {
					return err
				}
			}
		}

		for _, key := range keys {
			instance := key.Instance
			if _, ok := newM[instance]; !ok {
				continue
			}
			human := labels[instance]
			if human == "" {
				human = instance
			}

			if err := onInstance(runtime.ObserveEvent{InstanceID: instance, HumanReadableID: human, Added: true}); err != nil {
				return err
			}
			announced[instance] = struct{}{}
		}

		if opts.OneShot {
			return nil
		}

		time.Sleep(2 * time.Second)
	}
}

func (r k8sRuntime) DeleteRecursively(ctx context.Context) error {
	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return err
	}

	return tasks.Action("kubernetes.namespace.delete").Arg("namespace", r.ns()).Run(ctx, func(ctx context.Context) error {
		var grace int64 = 0
		return cli.CoreV1().Namespaces().Delete(ctx, r.ns(), metav1.DeleteOptions{
			GracePeriodSeconds: &grace,
		})
	})
}
