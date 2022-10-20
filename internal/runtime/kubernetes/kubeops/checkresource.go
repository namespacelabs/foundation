// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"fmt"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/kubeobserver"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

type Object interface {
	GetObjectKind() schema.ObjectKind
}

func fetchResource(ctx context.Context, cluster kubedef.KubeCluster, description string, resource schema.GroupVersionResource, name, namespace string, scope []fnschema.PackageName) (*unstructured.Unstructured, error) {
	return tasks.Return(ctx, tasks.Action("kubernetes.get").Scope(scope...).
		HumanReadablef("Check: "+description).
		Arg("resource", resource.Resource).
		Arg("name", name).
		Arg("namespace", namespace), func(ctx context.Context) (*unstructured.Unstructured, error) {
		client, err := client.MakeGroupVersionBasedClient(ctx, cluster.PreparedClient().RESTConfig, resource.GroupVersion())
		if err != nil {
			return nil, err
		}

		opts := metav1.GetOptions{}
		req := client.Get()
		if namespace != "" {
			req.Namespace(namespace)
		}

		r := req.Resource(resource.Resource).
			Name(name).
			Body(&opts)

		if OutputKubeApiURLs {
			fmt.Fprintf(console.Debug(ctx), "kubernetes: api get call %q\n", r.URL())
		}

		var res unstructured.Unstructured
		if err := r.Do(ctx).Into(&res); err != nil {
			if errors.IsNotFound(err) {
				tasks.Attachments(ctx).AddResult("exists", false)
				return nil, nil
			} else {
				return nil, err
			}
		}

		tasks.Attachments(ctx).AddResult("exists", true)
		return &res, nil
	})
}

func resolveResource(ctx context.Context, cluster kubedef.KubeCluster, gvk schema.GroupVersionKind) (*schema.GroupVersionResource, error) {
	return tasks.Return(ctx, tasks.Action("kubernetes.resolve-resource").Arg("gvk", gvk.String()).LogLevel(1),
		func(ctx context.Context) (*schema.GroupVersionResource, error) {
			state := "first-query"

			for {
				// Optimistic path -- assume the target Kind exists and is available.
				resource, queryErr := resolveResourceImpl(ctx, cluster, gvk)
				if queryErr == nil {
					return resource, nil
				}

				if state == "final-attempt" || !meta.IsNoMatchError(queryErr) {
					return nil, queryErr
				}

				// We don't know yet why, but didn't get a match. Perhaps need a rest mapper cache refresh?
				if state == "first-query" {
					state = "cache-reset-query"

					if err := resetCRDCache(ctx, cluster); err != nil {
						return nil, err
					}
				} else {
					state = "final-attempt"
					cli, err := apiextensionsv1.NewForConfig(cluster.PreparedClient().RESTConfig)
					if err != nil {
						return nil, fnerrors.InternalError("failed to prepare apiextensionsv1 client when querying resource: %v: %w", gvk, err)
					}

					// Still got a no match with a clean cache; could be that the target CRD is still being installed.
					crds, err := cli.CustomResourceDefinitions().List(ctx, metav1.ListOptions{})
					if err != nil {
						return nil, fnerrors.InternalError("failed to query crd when querying resource:  %v: %w", gvk, err)
					}

					var matching *v1.CustomResourceDefinition
					for _, crd := range crds.Items {
						if gvk.Kind == crd.Spec.Names.Kind {
							matching = &crd
							break
						}
					}

					if matching == nil {
						// No CRD matches the Kind we're attempting to resolve: return the original error.
						return nil, queryErr
					}

					if err := kubeobserver.WaitForCondition[*apiextensionsv1.ApiextensionsV1Client](
						ctx, cli, tasks.Action("kubernetes.wait-for-crd").Arg("crd", matching.Name),
						waitForCRD{name: matching.Name, plural: matching.Spec.Names.Plural}); err != nil {
						return nil, err
					}
				}
			}
		})
}

func resolveResourceImpl(ctx context.Context, cluster kubedef.KubeCluster, gvk schema.GroupVersionKind) (*schema.GroupVersionResource, error) {
	restMapper, err := cluster.EnsureState(ctx, kubernetes.RestmapperStateKey)
	if err != nil {
		return nil, err
	}

	resource, err := restMapper.(meta.RESTMapper).RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return nil, err
	}

	tasks.Attachments(ctx).AddResult("resource", resource.Resource.Resource)

	return &resource.Resource, nil
}

type waitForCRD struct {
	name   string
	plural string
}

func (w waitForCRD) Prepare(context.Context, *apiextensionsv1.ApiextensionsV1Client) error {
	return nil
}

func (w waitForCRD) Poll(ctx context.Context, cli *apiextensionsv1.ApiextensionsV1Client) (bool, error) {
	crd, err := cli.CustomResourceDefinitions().Get(ctx, w.name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	// XXX check conditions instead.
	return crd.Status.AcceptedNames.Plural == w.plural, nil
}
