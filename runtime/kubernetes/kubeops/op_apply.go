// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"fmt"

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
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	kobs "namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerApply() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.SerializedInvocation, apply *kubedef.OpApply) (*ops.HandleResult, error) {
		if apply.BodyJson == "" {
			return nil, fnerrors.InternalError("%s: apply.Body is required", d.Description)
		}

		header, err := kubeparser.Header([]byte(apply.BodyJson))
		if err != nil {
			return nil, fnerrors.BadInputError("%s: kubernetes.apply: failed to parse resource: %w", d.Description, err)
		}

		gv := header.GetObjectKind().GroupVersionKind().GroupVersion()

		if gv.Version == "" {
			return nil, fnerrors.InternalError("%s: APIVersion is required", d.Description)
		}

		restcfg, err := client.ResolveConfig(ctx, env)
		if err != nil {
			return nil, err
		}

		resourceName := apply.GetResourceClass().GetResource()
		if resourceName == "" {
			resourceName = kubeparser.ResourceEndpointFromKind(header.Kind)
			if resourceName == "" {
				return nil, fnerrors.InternalError("don't know the resource mapping for %q", header.Kind)
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
			Arg("name", header.Name).
			Arg("namespace", header.Namespace).RunWithOpts(ctx, tasks.RunOpts{
			Wait: func(ctx context.Context) (bool, error) {
				// CRDs are funky in that they take a moment to apply, and
				// before that happens the api server doesn't accept patches
				// for them. So we first check if the CRD does exist, and we
				// wait for its paths to become ready.
				// XXX we should have metadata that identifies the resource class as a CRD.
				if apply.ResourceClass != nil && apply.ResourceClass.GetGroup() == "k8s.namespacelabs.dev" {
					crd := fmt.Sprintf("%s.%s", apply.ResourceClass.GetResource(), apply.ResourceClass.GetGroup())

					cli, err := apiextensionsv1.NewForConfig(restcfg)
					if err != nil {
						return false, err
					}

					if err := kobs.WaitForCondition[*apiextensionsv1.ApiextensionsV1Client](
						ctx, cli, tasks.Action("kubernetes.wait-for-crd").Arg("crd", crd),
						waitForCRD{apply.ResourceClass.Resource, crd}); err != nil {
						return false, err
					}
				}

				return false, nil
			},
			Run: func(ctx context.Context) error {
				client, err := client.MakeGroupVersionBasedClient(ctx, gv, restcfg)
				if err != nil {
					return err
				}

				patchOpts := kubedef.Ego().ToPatchOptions()
				req := client.Patch(types.ApplyPatchType)
				if header.Namespace != "" {
					req = req.Namespace(header.Namespace)
				}

				prepReq := req.Resource(resourceName).
					Name(header.Name).
					VersionedParams(&patchOpts, metav1.ParameterCodec).
					Body([]byte(apply.BodyJson))

				if OutputKubeApiURLs {
					fmt.Fprintf(console.Debug(ctx), "kubernetes: api patch call %q\n", prepReq.URL())
				}

				return prepReq.Do(ctx).Into(&res)
			}}); err != nil {
			return nil, fnerrors.InvocationError("%s: failed to apply: %w", d.Description, err)
		}

		if header.Namespace == kubedef.AdminNamespace {
			// don't wait for changes to admin namespace
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
				RestConfig:         restcfg,
				Namespace:          header.Namespace,
				Name:               header.Name,
				ExpectedGeneration: generation,
				ConditionType:      apply.CheckGenerationCondition.Type,
				ResourceClass:      apply.ResourceClass,
			}.WaitUntilReady}}, nil
		}

		switch header.Kind {
		case "Deployment", "StatefulSet":
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
						RestConfig:   restcfg,
						Description:  d.Description,
						Namespace:    header.Namespace,
						Name:         header.Name,
						ResourceKind: header.Kind,
						Scope:        sc,
						PreviousGen:  observedGen,
						ExpectedGen:  generation,
					}
					waiters = append(waiters, w.WaitUntilReady)
				}
				return &ops.HandleResult{
					Waiters: waiters,
				}, nil
			} else {
				fmt.Fprintf(console.Warnings(ctx), "missing generation data from %s: %v / %v [found1=%v found2=%v]\n",
					header.Kind, err1, err2, found1, found2)
			}

		case "Pod":
			var waiters []ops.Waiter
			for _, sc := range scope {
				sc := sc // Close sc.
				waiters = append(waiters, func(ctx context.Context, ch chan ops.Event) error {
					if ch != nil {
						defer close(ch)
					}

					cli, err := k8s.NewForConfig(restcfg)
					if err != nil {
						return err
					}

					return kobs.WaitForCondition(ctx, cli, tasks.Action(runtime.TaskServerStart).Scope(sc),
						kobs.WaitForPodConditition(kobs.ResolvePod(header.Namespace, header.Name),
							func(ps v1.PodStatus) (bool, error) {
								ev := ops.Event{
									ResourceID:          fmt.Sprintf("%s/%s", header.Namespace, header.Name),
									Kind:                header.Kind,
									Category:            "Servers deployed",
									Scope:               sc,
									Ready:               ops.NotReady,
									ImplMetadata:        ps,
									RuntimeSpecificHelp: fmt.Sprintf("kubectl -n %s describe pod %s", header.Namespace, header.Name),
								}

								ev.WaitStatus = append(ev.WaitStatus, kobs.WaiterFromPodStatus(header.Namespace, header.Name, ps))

								ready, _ := kobs.MatchPodCondition(v1.PodReady)(ps)
								if ready {
									ev.Ready = ops.Ready
								}
								if ch != nil {
									ch <- ev
								}
								return ready, nil
							}))
				})

			}

			return &ops.HandleResult{
				Waiters: waiters,
			}, nil

		case "Namespace":
			// Special-case namespace, we don't return until the default service account has been created.
			c, err := k8s.NewForConfig(restcfg)
			if err != nil {
				return nil, err
			}

			if err := tasks.Action("kubernetes.apply.wait-for-namespace").Arg("name", header.Name).Run(ctx, func(ctx context.Context) error {
				w, err := c.CoreV1().ServiceAccounts(header.Name).Watch(ctx, metav1.ListOptions{})
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
			}); err != nil {
				return nil, err
			}
		}

		return nil, nil
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
