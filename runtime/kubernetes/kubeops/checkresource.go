// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
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
	return tasks.Return(ctx, tasks.Action("kubernetes.resolve-resource").Arg("gvk", gvk.String()), func(ctx context.Context) (*schema.GroupVersionResource, error) {
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
	})
}
