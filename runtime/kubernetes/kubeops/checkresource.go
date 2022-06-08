// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func checkResourceExists(ctx context.Context, restcfg *rest.Config, description string, resource client.ResourceClassLike, name, namespace string, scope []schema.PackageName) (bool, error) {
	var exists bool
	// XXX this is racy here, we need to have a loop and a callback for contents.
	if err := tasks.Action("kubernetes.get").Scope(scope...).
		HumanReadablef("Check: "+description).
		Arg("resource", resourceName(resource)).
		Arg("name", name).
		Arg("namespace", namespace).Run(ctx, func(ctx context.Context) error {
		client, err := client.MakeResourceSpecificClient(ctx, resource, restcfg)
		if err != nil {
			return err
		}

		opts := metav1.GetOptions{}
		req := client.Get()
		if namespace != "" {
			req.Namespace(namespace)
		}

		if err := req.Resource(resourceName(resource)).
			Name(name).
			Body(&opts).
			Do(ctx).Error(); err != nil {
			if errors.IsNotFound(err) {
				return nil
			} else {
				return err
			}
		}

		exists = true
		return nil
	}); err != nil {
		return false, err
	}

	return exists, nil
}

func resourceName(r client.ResourceClassLike) string {
	if klass := r.GetResourceClass(); klass != nil {
		return klass.Resource
	}

	return r.GetResource()
}
