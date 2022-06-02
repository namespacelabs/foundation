// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func registerCreate() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.SerializedInvocation, create *kubedef.OpCreate) (*ops.HandleResult, error) {
		if create.Resource == "" {
			return nil, fnerrors.InternalError("%s: create.Resource is required", d.Description)
		}

		if create.Name == "" {
			return nil, fnerrors.InternalError("%s: create.Name is required", d.Description)
		}

		if create.IfMissing {
			exists, err := checkResourceExists(ctx, env, d.Description, create.Resource, create.Name,
				create.Namespace, schema.PackageNames(d.Scope...))
			if err != nil {
				return nil, err
			}

			if exists {
				return nil, nil // Nothing to do.
			}
		}

		if err := tasks.Action("kubernetes.create").Scope(schema.PackageNames(d.Scope...)...).
			HumanReadablef(d.Description).
			Arg("resource", create.Resource).
			Arg("name", create.Name).
			Arg("namespace", create.Namespace).Run(ctx, func(ctx context.Context) error {
			restcfg, err := client.ResolveConfig(env)
			if err != nil {
				return err
			}

			client, err := client.MakeResourceSpecificClient(create.Resource, restcfg)
			if err != nil {
				return err
			}

			opts := metav1.CreateOptions{}
			req := client.Post()
			if create.Namespace != "" {
				req.Namespace(create.Namespace)
			}

			return req.Resource(create.Resource).
				VersionedParams(&opts, scheme.ParameterCodec).
				Body([]byte(create.BodyJson)).
				Do(ctx).Error()
		}); err != nil && !errors.IsNotFound(err) {
			return nil, fnerrors.InvocationError("%s: failed to create: %w", d.Description, err)
		}

		return nil, nil
	})
}
