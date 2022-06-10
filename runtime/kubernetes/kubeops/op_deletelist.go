// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerDeleteList() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.SerializedInvocation, deleteList *kubedef.OpDeleteList) (*ops.HandleResult, error) {
		if deleteList.Resource == "" {
			return nil, fnerrors.InternalError("%s: deleteList.Resource is required", d.Description)
		}

		if deleteList.Namespace == "" {
			return nil, fnerrors.InternalError("%s: deleteList.Namespace is required", d.Description)
		}

		if err := tasks.Action("kubernetes.delete-collection").Scope(schema.PackageNames(d.Scope...)...).
			HumanReadablef(d.Description).
			Arg("resource", resourceName(deleteList)).
			Arg("selector", deleteList.LabelSelector).
			Arg("namespace", deleteList.Namespace).Run(ctx, func(ctx context.Context) error {
			restcfg, err := client.ResolveConfig(ctx, env)
			if err != nil {
				return err
			}

			client, err := client.MakeResourceSpecificClient(ctx, deleteList, restcfg)
			if err != nil {
				return err
			}

			listOpts := metav1.ListOptions{LabelSelector: deleteList.LabelSelector}

			get := client.Get().
				Namespace(deleteList.Namespace).
				Resource(resourceName(deleteList)).
				VersionedParams(&listOpts, metav1.ParameterCodec)

			if OutputKubeApiURLs {
				fmt.Fprintf(console.Debug(ctx), "kubernetes: api get call %q\n", get.URL())
			}

			var res unstructured.UnstructuredList
			if err := get.Do(ctx).Into(&res); err != nil {
				return err
			}

			var names []string
			if err := res.EachListItem(func(o k8sruntime.Object) error {
				if u, ok := o.(*unstructured.Unstructured); ok {
					names = append(names, u.GetName())
				}
				return nil
			}); err != nil {
				return err
			}

			tasks.Attachments(ctx).AddResult("names", names)

			return res.EachListItem(func(o k8sruntime.Object) error {
				u, ok := o.(*unstructured.Unstructured)
				if !ok {
					return fmt.Errorf("expected an unstructured value")
				}

				opts := metav1.DeleteOptions{}

				r := client.Delete().
					Namespace(deleteList.Namespace).
					Resource(resourceName(deleteList)).
					Name(u.GetName()).
					Body(&opts)

				if OutputKubeApiURLs {
					fmt.Fprintf(console.Debug(ctx), "kubernetes: api delete call %q\n", r.URL())
				}

				return r.Do(ctx).Error()
			})
		}); err != nil {
			return nil, fnerrors.InvocationError("%s: %w", d.Description, err)
		}

		return nil, nil
	})
}
