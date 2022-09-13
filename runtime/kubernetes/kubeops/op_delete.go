// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerDelete() {
	ops.RegisterFunc(func(ctx context.Context, env planning.Context, d *schema.SerializedInvocation, delete *kubedef.OpDelete) (*ops.HandleResult, error) {
		if delete.Resource == "" {
			return nil, fnerrors.InternalError("%s: delete.Resource is required", d.Description)
		}

		if delete.Name == "" {
			return nil, fnerrors.InternalError("%s: delete.Name is required", d.Description)
		}

		cluster, err := kubedef.InjectedKubeCluster(ctx)
		if err != nil {
			return nil, err
		}

		if err := tasks.Action("kubernetes.delete").Scope(schema.PackageNames(d.Scope...)...).
			HumanReadablef(d.Description).
			Arg("resource", resourceName(delete)).
			Arg("name", delete.Name).
			Arg("namespace", delete.Namespace).Run(ctx, func(ctx context.Context) error {
			client, err := client.MakeResourceSpecificClient(ctx, delete, cluster.RESTConfig())
			if err != nil {
				return err
			}

			opts := metav1.DeleteOptions{}
			req := client.Delete()
			if delete.Namespace != "" {
				req.Namespace(delete.Namespace)
			}

			r := req.Resource(resourceName(delete)).
				Name(delete.Name).
				Body(&opts)

			if OutputKubeApiURLs {
				fmt.Fprintf(console.Debug(ctx), "kubernetes: api delete call %q\n", r.URL())
			}

			return r.Do(ctx).Error()
		}); err != nil && !errors.IsNotFound(err) {
			return nil, fnerrors.InvocationError("%s: failed to delete: %w", d.Description, err)
		}

		return nil, nil
	})
}
