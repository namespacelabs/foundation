// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func fetchResource(ctx context.Context, restcfg *rest.Config, description string, resource client.ResourceClassLike, name, namespace string, scope []schema.PackageName) (*unstructured.Unstructured, error) {
	return tasks.Return(ctx, tasks.Action("kubernetes.get").Scope(scope...).
		HumanReadablef("Check: "+description).
		Arg("resource", resourceName(resource)).
		Arg("name", name).
		Arg("namespace", namespace), func(ctx context.Context) (*unstructured.Unstructured, error) {
		client, err := client.MakeResourceSpecificClient(ctx, resource, restcfg)
		if err != nil {
			return nil, err
		}

		opts := metav1.GetOptions{}
		req := client.Get()
		if namespace != "" {
			req.Namespace(namespace)
		}

		r := req.Resource(resourceName(resource)).
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

func resourceName(r client.ResourceClassLike) string {
	if klass := r.GetResourceClass(); klass != nil {
		return klass.Resource
	}

	return r.GetResource()
}
