// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeops

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/exp/slices"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	k8s "k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	kobs "namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/tasks"
)

func registerApply() {
	execution.RegisterVFuncs(execution.VFuncs[*kubedef.OpApply, *parsedApply]{
		Parse: func(ctx context.Context, def *fnschema.SerializedInvocation, apply *kubedef.OpApply) (*parsedApply, error) {
			if apply.BodyJson == "" {
				return nil, fnerrors.InternalError("apply.Body is required")
			}

			var parsed unstructured.Unstructured
			if err := json.Unmarshal([]byte(apply.BodyJson), &parsed); err != nil {
				return nil, fnerrors.BadInputError("kubernetes.apply: failed to parse resource: %w", err)
			}

			return &parsedApply{obj: &parsed, spec: apply}, nil
		},

		Handle: func(ctx context.Context, d *fnschema.SerializedInvocation, parsed *parsedApply) (*execution.HandleResult, error) {
			return apply(ctx, d.Description, fnschema.PackageNames(d.Scope...), parsed.obj, parsed.spec, nil)
		},

		PlanOrder: func(ctx context.Context, apply *parsedApply) (*fnschema.ScheduleOrder, error) {
			return kubedef.PlanOrder(apply.obj.GroupVersionKind(), apply.obj.GetNamespace(), apply.obj.GetName()), nil
		},
	})
}

type parsedApply struct {
	obj  *unstructured.Unstructured
	spec *kubedef.OpApply
}

func apply(ctx context.Context, desc string, scope []fnschema.PackageName, obj kubedef.Object, spec *kubedef.OpApply, ch chan *orchestration.Event) (*execution.HandleResult, error) {
	gv := obj.GroupVersionKind().GroupVersion()
	if gv.Version == "" {
		return nil, fnerrors.InternalError("%s: APIVersion is required", desc)
	}

	cluster, err := kubedef.InjectedKubeCluster(ctx)
	if err != nil {
		return nil, err
	}

	restcfg := cluster.PreparedClient().RESTConfig

	var resource *schema.GroupVersionResource
	var res unstructured.Unstructured

	action := tasks.Action("kubernetes.apply").
		Scope(scope...).
		HumanReadablef(desc).
		Arg("name", obj.GetName())

	if ns := obj.GetNamespace(); ns != "" {
		action = action.Arg("namespace", ns)
	}

	if err := action.RunWithOpts(ctx, tasks.RunOpts{
		Wait: func(ctx context.Context) (bool, error) {
			var err error
			resource, err = resolveResource(ctx, cluster, obj.GroupVersionKind())
			if err != nil {
				return false, err
			}

			tasks.Attachments(ctx).AddResult("resource", resource.Resource)

			// Creating Deployments and Statefulsets that refer to the
			// default service account, requires that the default service
			// account actually exists. And creating the default service
			// account takes a bit of time after creating a namespace.
			waitOnNamespace, err := requiresWaitForNamespace(obj, spec)
			if err != nil {
				return false, fnerrors.InternalError("failed to determine object namespace: %w", err)
			}

			if waitOnNamespace {
				if err := waitForDefaultServiceAccount(ctx, cluster.PreparedClient().Clientset, obj.GetNamespace()); err != nil {
					return false, err
				}
			}

			// On ephemeral environments, e.g. tests, we don't wait for an
			// ingress controller to be present, before installing ingress
			// objects. This is because we sometimes run in environments where
			// there's no controller installed (e.g. in ephemeral nscloud
			// clusters). And tests don't (yet) exercise ingress objects.
			if obj.GroupVersionKind().GroupVersion().String() == "networking.k8s.io/v1" && obj.GroupVersionKind().Kind == "Ingress" {
				clusterns, _ := kubedef.InjectedKubeClusterNamespace(ctx)
				// We check for clusterns presence as it's not present when
				// deploying the ingress controller itself.
				if clusterns != nil && !clusterns.KubeConfig().Environment.Ephemeral {
					if err := ingress.EnsureState(ctx, cluster); err != nil {
						return false, err
					}
				}
			}

			return false, nil
		},
		Run: func(ctx context.Context) error {
			client, err := client.MakeGroupVersionBasedClient(ctx, restcfg, resource.GroupVersion())
			if err != nil {
				return err
			}

			patchOpts := kubedef.Ego().ToPatchOptions()
			req := client.Patch(types.ApplyPatchType)
			if obj.GetNamespace() != "" {
				req = req.Namespace(obj.GetNamespace())
			}

			prepReq := req.Resource(resource.Resource).
				Name(obj.GetName()).
				VersionedParams(&patchOpts, metav1.ParameterCodec).
				Body([]byte(spec.BodyJson))

			if OutputKubeApiURLs {
				fmt.Fprintf(console.Debug(ctx), "kubernetes: api patch call %q\n", prepReq.URL())
			}

			return prepReq.Do(ctx).Into(&res)
		}}); err != nil {
		return nil, fnerrors.InvocationError("%s: failed to apply: %w", desc, err)
	}

	if err := checkResetCRDCache(ctx, cluster, obj.GroupVersionKind()); err != nil {
		return nil, err
	}

	if spec.CheckGenerationCondition.GetType() != "" {
		generation, found1, err1 := unstructured.NestedInt64(res.Object, "metadata", "generation")
		if err1 != nil {
			return nil, fnerrors.InternalError("failed to wait on resource: %w", err)
		}
		if !found1 {
			return nil, fnerrors.InternalError("failed to wait on resource: no metadata.generation")
		}

		return &execution.HandleResult{
			Waiter: kobs.WaitOnGenerationCondition{
				RestConfig:         restcfg,
				Namespace:          obj.GetNamespace(),
				Name:               obj.GetName(),
				ExpectedGeneration: generation,
				ConditionType:      spec.CheckGenerationCondition.Type,
				Resource:           *resource,
			}.WaitUntilReady,
		}, nil
	}

	if spec.InhibitEvents {
		return nil, nil
	}

	if ch != nil {
		switch {
		case kubedef.IsDeployment(obj), kubedef.IsStatefulSet(obj), kubedef.IsPod(obj):
			ev := kobs.PrepareEvent(obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName(), desc, spec.Deployable.GetPackageName())
			ev.WaitStatus = append(ev.WaitStatus, &orchestration.Event_WaitStatus{
				Description: "Commited...",
			})
			ch <- ev
		}
	}

	switch {
	case kubedef.IsDeployment(obj), kubedef.IsStatefulSet(obj):
		generation, found1, err1 := unstructured.NestedInt64(res.Object, "metadata", "generation")
		observedGen, found2, err2 := unstructured.NestedInt64(res.Object, "status", "observedGeneration")
		if err2 != nil || !found2 {
			observedGen = 0 // Assume no generation exists.
		}

		// XXX print a warning if expected fields are missing.
		if err1 == nil && found1 {
			w := kobs.WaitOnResource{
				RestConfig:       restcfg,
				Description:      desc,
				Namespace:        obj.GetNamespace(),
				Name:             obj.GetName(),
				GroupVersionKind: obj.GroupVersionKind(),
				PreviousGen:      observedGen,
				ExpectedGen:      generation,
			}

			if spec.Deployable != nil {
				w.Scope = fnschema.PackageName(spec.Deployable.GetPackageName())
			}

			return &execution.HandleResult{Waiter: w.WaitUntilReady}, nil
		} else {
			fmt.Fprintf(console.Warnings(ctx), "missing generation data from %s: %v / %v [found1=%v found2=%v]\n",
				obj.GroupVersionKind().Kind, err1, err2, found1, found2)
		}

	case kubedef.IsPod(obj):
		return &execution.HandleResult{
			Waiter: func(ctx context.Context, ch chan *orchestration.Event) error {
				if ch != nil {
					defer close(ch)
				}

				return kobs.WaitForCondition(ctx, cluster.PreparedClient().Clientset, tasks.Action("pod.wait").Scope(scope...),
					kobs.WaitForPodConditition(obj.GetNamespace(), kobs.PickPod(obj.GetName()),
						func(ps v1.PodStatus) (bool, error) {
							meta, err := json.Marshal(ps)
							if err != nil {
								return false, fnerrors.InternalError("failed to marshal pod status: %w", err)
							}

							ev := kobs.PrepareEvent(obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName(), desc, spec.Deployable.GetPackageName())
							ev.ImplMetadata = meta
							ev.WaitStatus = append(ev.WaitStatus, kobs.WaiterFromPodStatus(obj.GetNamespace(), obj.GetName(), ps))

							var done bool
							if ps.Phase == v1.PodFailed || ps.Phase == v1.PodSucceeded {
								// If the pod is finished, then don't wait further.
								done = true
							} else {
								done, _ = kobs.MatchPodCondition(v1.PodReady)(ps)
							}

							if done {
								ev.Ready = orchestration.Event_READY
							}

							if ch != nil {
								ch <- ev
							}
							return done, nil
						}))
			}}, nil

	case kubedef.IsService(obj):
		pkg, ok := obj.GetAnnotations()[kubedef.K8sServicePackageName]
		if ok && !slices.Contains(scope, fnschema.PackageName(pkg)) {
			// Refine scope with service package for understandability.
			scope = append(scope, fnschema.PackageName(pkg))
		}

		return &execution.HandleResult{
			Waiter: func(ctx context.Context, ch chan *orchestration.Event) error {
				if ch != nil {
					defer close(ch)
				}

				return kobs.WaitForCondition(ctx, cluster.PreparedClient().Clientset, tasks.Action("service.wait").Scope(scope...),
					kobs.WaitForService(obj.GetNamespace(), obj.GetName(),
						func(pods []v1.Pod, err error) bool {
							ev := kobs.PrepareEvent(obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName(), desc, pkg)

							for _, p := range pods {
								ev.WaitStatus = append(ev.WaitStatus, kobs.WaiterFromPodStatus(p.Namespace, p.Name, p.Status))
							}

							if err == nil {
								ev.Ready = orchestration.Event_READY
							} else {
								ev.WaitStatus = append(ev.WaitStatus, kobs.WaiterFromServiceErr(err))
							}

							if ch != nil {
								ch <- ev
							}

							return ev.Ready == orchestration.Event_READY
						}))
			},
		}, nil
	}

	return nil, nil
}

func requiresWaitForNamespace(obj kubedef.Object, spec *kubedef.OpApply) (bool, error) {
	switch {
	case kubedef.IsDeployment(obj):
		// XXX change to unstructured lookups.
		var d appsv1.Deployment
		if err := json.Unmarshal([]byte(spec.BodyJson), &d); err != nil {
			return false, err
		}
		if d.Spec.Template.Spec.ServiceAccountName == "" || d.Spec.Template.Spec.ServiceAccountName == "default" {
			return true, nil
		}

	case kubedef.IsStatefulSet(obj):
		var d appsv1.StatefulSet
		if err := json.Unmarshal([]byte(spec.BodyJson), &d); err != nil {
			return false, err
		}
		if d.Spec.Template.Spec.ServiceAccountName == "" || d.Spec.Template.Spec.ServiceAccountName == "default" {
			return true, nil
		}

	case kubedef.IsPod(obj):
		var d v1.Pod
		if err := json.Unmarshal([]byte(spec.BodyJson), &d); err != nil {
			return false, err
		}
		if d.Spec.ServiceAccountName == "" || d.Spec.ServiceAccountName == "default" {
			return true, nil
		}
	}

	return false, nil
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
