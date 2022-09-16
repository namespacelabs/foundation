// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	kobs "namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerApply() {
	ops.RegisterFunc(func(ctx context.Context, d *schema.SerializedInvocation, apply *kubedef.OpApply) (*ops.HandleResult, error) {
		if apply.BodyJson == "" {
			return nil, fnerrors.InternalError("%s: apply.Body is required", d.Description)
		}

		var parsed unstructured.Unstructured
		if err := json.Unmarshal([]byte(apply.BodyJson), &parsed); err != nil {
			return nil, fnerrors.BadInputError("%s: kubernetes.apply: failed to parse resource: %w", d.Description, err)
		}

		gv := parsed.GroupVersionKind().GroupVersion()
		if gv.Version == "" {
			return nil, fnerrors.InternalError("%s: APIVersion is required", d.Description)
		}

		cluster, err := kubedef.InjectedKubeCluster(ctx)
		if err != nil {
			return nil, err
		}

		resourceName := apply.GetResourceClass().GetResource()
		if resourceName == "" {
			resourceName = kubeparser.ResourceEndpointFromKind(&parsed)
			if resourceName == "" {
				return nil, fnerrors.InternalError("don't know the resource mapping for %q", parsed.GetKind())
			}
		}

		if rc := apply.GetResourceClass(); rc != nil {
			gv = kubeschema.GroupVersion{Group: rc.Group, Version: rc.Version}
		}

		scope := schema.PackageNames(d.Scope...)
		var res unstructured.Unstructured
		if err := tasks.Action("kubernetes.apply").Scope(scope...).
			HumanReadablef(d.Description).
			Arg("resource", resourceName).
			Arg("name", parsed.GetName()).
			Arg("namespace", parsed.GetNamespace()).RunWithOpts(ctx, tasks.RunOpts{
			Wait: func(ctx context.Context) (bool, error) {
				// CRDs are funky in that they take a moment to apply, and
				// before that happens the api server doesn't accept patches
				// for them. So we first check if the CRD does exist, and we
				// wait for its paths to become ready.
				// XXX we should have metadata that identifies the resource class as a CRD.
				if apply.ResourceClass != nil && apply.ResourceClass.GetGroup() == "k8s.namespacelabs.dev" {
					crd := fmt.Sprintf("%s.%s", apply.ResourceClass.GetResource(), apply.ResourceClass.GetGroup())

					cli, err := apiextensionsv1.NewForConfig(cluster.RESTConfig())
					if err != nil {
						return false, err
					}

					if err := kobs.WaitForCondition[*apiextensionsv1.ApiextensionsV1Client](
						ctx, cli, tasks.Action("kubernetes.wait-for-crd").Arg("crd", crd),
						waitForCRD{apply.ResourceClass.Resource, crd}); err != nil {
						return false, err
					}
				}

				// Creating Deployments and Statefulsets that refer to the
				// default service account, requires that the default service
				// account actually exists. And creating the default service
				// account takes a bit of time after creating a namespace.
				var waitOnNamespace string
				switch {
				case kubedef.IsDeployment(&parsed):
					var d appsv1.Deployment
					if err := json.Unmarshal([]byte(apply.BodyJson), &d); err != nil {
						return false, err
					}
					if d.Spec.Template.Spec.ServiceAccountName == "" || d.Spec.Template.Spec.ServiceAccountName == "default" {
						waitOnNamespace = parsed.GetNamespace()
					}

				case kubedef.IsStatefulSet(&parsed):
					var d appsv1.StatefulSet
					if err := json.Unmarshal([]byte(apply.BodyJson), &d); err != nil {
						return false, err
					}
					if d.Spec.Template.Spec.ServiceAccountName == "" || d.Spec.Template.Spec.ServiceAccountName == "default" {
						waitOnNamespace = parsed.GetNamespace()
					}

				case kubedef.IsPod(&parsed):
					var d v1.Pod
					if err := json.Unmarshal([]byte(apply.BodyJson), &d); err != nil {
						return false, err
					}
					if d.Spec.ServiceAccountName == "" || d.Spec.ServiceAccountName == "default" {
						waitOnNamespace = parsed.GetNamespace()
					}
				}

				if waitOnNamespace != "" {
					if err := waitForDefaultServiceAccount(ctx, cluster.Client(), waitOnNamespace); err != nil {
						return false, err
					}
				}

				if parsed.GetAPIVersion() == "networking.k8s.io/v1" && parsed.GetKind() == "Ingress" {
					if err := ingress.EnsureState(ctx, cluster); err != nil {
						return false, err
					}
				}

				return false, nil
			},
			Run: func(ctx context.Context) error {
				client, err := client.MakeGroupVersionBasedClient(ctx, gv, cluster.RESTConfig())
				if err != nil {
					return err
				}

				patchOpts := kubedef.Ego().ToPatchOptions()
				req := client.Patch(types.ApplyPatchType)
				if parsed.GetNamespace() != "" {
					req = req.Namespace(parsed.GetNamespace())
				}

				prepReq := req.Resource(resourceName).
					Name(parsed.GetName()).
					VersionedParams(&patchOpts, metav1.ParameterCodec).
					Body([]byte(apply.BodyJson))

				if OutputKubeApiURLs {
					fmt.Fprintf(console.Debug(ctx), "kubernetes: api patch call %q\n", prepReq.URL())
				}

				return prepReq.Do(ctx).Into(&res)
			}}); err != nil {
			return nil, fnerrors.InvocationError("%s: failed to apply: %w", d.Description, err)
		}

		if parsed.GetNamespace() == kubedef.AdminNamespace && !kubedef.HasFocusMark(parsed.GetLabels()) {
			// don't wait for changes to admin namespace, unless they are in focus
			return &ops.HandleResult{}, nil
		}

		if apply.CheckGenerationCondition.GetType() != "" {
			generation, found1, err1 := unstructured.NestedInt64(res.Object, "metadata", "generation")
			if err1 != nil {
				return nil, fnerrors.InternalError("failed to wait on resource: %w", err)
			}
			if !found1 {
				return nil, fnerrors.InternalError("failed to wait on resource: no metadata.generation")
			}

			return &ops.HandleResult{Waiters: []ops.Waiter{kobs.WaitOnGenerationCondition{
				RestConfig:         cluster.RESTConfig(),
				Namespace:          parsed.GetNamespace(),
				Name:               parsed.GetName(),
				ExpectedGeneration: generation,
				ConditionType:      apply.CheckGenerationCondition.Type,
				ResourceClass:      apply.ResourceClass,
			}.WaitUntilReady}}, nil
		}

		// XXX check gkv
		switch {
		case kubedef.IsDeployment(&parsed), kubedef.IsStatefulSet(&parsed):
			generation, found1, err1 := unstructured.NestedInt64(res.Object, "metadata", "generation")
			observedGen, found2, err2 := unstructured.NestedInt64(res.Object, "status", "observedGeneration")
			if err2 != nil || !found2 {
				observedGen = 0 // Assume no generation exists.
			}

			// XXX print a warning if expected fields are missing.
			if err1 == nil && found1 {
				var waiters []ops.Waiter
				for _, sc := range scope {
					w := kobs.WaitOnResource{
						RestConfig:       cluster.RESTConfig(),
						Description:      d.Description,
						Namespace:        parsed.GetNamespace(),
						Name:             parsed.GetName(),
						GroupVersionKind: parsed.GroupVersionKind(),
						Scope:            sc,
						PreviousGen:      observedGen,
						ExpectedGen:      generation,
					}
					waiters = append(waiters, w.WaitUntilReady)
				}
				return &ops.HandleResult{
					Waiters: waiters,
				}, nil
			} else {
				fmt.Fprintf(console.Warnings(ctx), "missing generation data from %s: %v / %v [found1=%v found2=%v]\n",
					parsed.GetKind(), err1, err2, found1, found2)
			}

		case kubedef.IsPod(&parsed):
			waiters := []ops.Waiter{func(ctx context.Context, ch chan *orchestration.Event) error {
				if ch != nil {
					defer close(ch)
				}

				return kobs.WaitForCondition(ctx, cluster.Client(), tasks.Action("pod.wait").Scope(scope...),
					kobs.WaitForPodConditition(
						parsed.GetNamespace(),
						kobs.PickPod(parsed.GetName()),
						func(ps v1.PodStatus) (bool, error) {
							meta, err := json.Marshal(ps)
							if err != nil {
								return false, fnerrors.InternalError("failed to marshal pod status: %w", err)
							}

							ev := &orchestration.Event{
								ResourceId:          fmt.Sprintf("%s/%s", parsed.GetNamespace(), parsed.GetName()),
								Kind:                parsed.GetKind(),
								Category:            "Servers deployed",
								Ready:               orchestration.Event_NOT_READY,
								ImplMetadata:        meta,
								RuntimeSpecificHelp: fmt.Sprintf("kubectl -n %s describe pod %s", parsed.GetNamespace(), parsed.GetName()),
							}

							// XXX this under-reports scope.
							if len(scope) > 0 {
								ev.Scope = scope[0].String()
							}

							ev.WaitStatus = append(ev.WaitStatus, kobs.WaiterFromPodStatus(parsed.GetNamespace(), parsed.GetName(), ps))

							var done bool
							if ps.Phase == v1.PodFailed || ps.Phase == v1.PodSucceeded {
								// If the pod is finished, then don't wait further.
								done = true
							} else {
								done, _ = kobs.MatchPodCondition(v1.PodReady)(ps)
								if done {
									ev.Ready = orchestration.Event_READY
								}
							}

							if ch != nil {
								ch <- ev
							}
							return done, nil
						}))
			}}

			return &ops.HandleResult{
				Waiters: waiters,
			}, nil
		}

		return nil, nil
	})
}

func waitForDefaultServiceAccount(ctx context.Context, c *k8s.Clientset, namespace string) error {
	return tasks.Action("kubernetes.apply.wait-for-namespace").Arg("name", namespace).Run(ctx, func(ctx context.Context) error {
		w, err := c.CoreV1().ServiceAccounts(namespace).Watch(ctx, metav1.ListOptions{})
		if err != nil {
			return fnerrors.InternalError("kubernetes: failed to wait until the namespace was ready: %w", err)
		}

		defer w.Stop()

		// Wait until the default service account has been created.
		for ev := range w.ResultChan() {
			if account, ok := ev.Object.(*v1.ServiceAccount); ok && ev.Type == watch.Added {
				if account.Name == "default" {
					return nil // Service account is ready.
				}
			}
		}

		return nil
	})
}

type waitForCRD struct {
	plural string
	crd    string
}

func (w waitForCRD) Prepare(context.Context, *apiextensionsv1.ApiextensionsV1Client) error {
	return nil
}

func (w waitForCRD) Poll(ctx context.Context, cli *apiextensionsv1.ApiextensionsV1Client) (bool, error) {
	crd, err := cli.CustomResourceDefinitions().Get(ctx, w.crd, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	return crd.Status.AcceptedNames.Plural == w.plural, nil
}
