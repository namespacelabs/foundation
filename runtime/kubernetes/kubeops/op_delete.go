// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerDelete() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.SerializedInvocation, delete *kubedef.OpDelete) (*ops.HandleResult, error) {
		if delete.Resource == "" {
			return nil, fnerrors.InternalError("%s: delete.Resource is required", d.Description)
		}

		if delete.Name == "" {
			return nil, fnerrors.InternalError("%s: delete.Name is required", d.Description)
		}

		if err := tasks.Action("kubernetes.delete").Scope(schema.PackageNames(d.Scope...)...).
			HumanReadablef(d.Description).
			Arg("resource", delete.Resource).
			Arg("name", delete.Name).
			Arg("namespace", delete.Namespace).Run(ctx, func(ctx context.Context) error {
			restcfg, err := client.ResolveConfig(ctx, env)
			if err != nil {
				return err
			}

			client, err := client.MakeResourceSpecificClient(delete.Resource, restcfg)
			if err != nil {
				return err
			}

			opts := metav1.DeleteOptions{}
			req := client.Delete()
			if delete.Namespace != "" {
				req.Namespace(delete.Namespace)
			}

			return req.Resource(delete.Resource).
				Name(delete.Name).
				Body(&opts).
				Do(ctx).Error()
		}); err != nil && !errors.IsNotFound(err) {
			return nil, fnerrors.InvocationError("%s: failed to delete: %w", d.Description, err)
		}

		return nil, nil
	})
}
