// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/console"
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

		restcfg, err := client.ResolveConfig(ctx, env)
		if err != nil {
			return nil, fnerrors.New("failed to resolve config: %w", err)
		}

		if create.SkipIfAlreadyExists {
			exists, err := checkResourceExists(ctx, restcfg, d.Description, create, create.Name,
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
			Arg("resource", resourceName(create)).
			Arg("name", create.Name).
			Arg("namespace", create.Namespace).Run(ctx, func(ctx context.Context) error {
			client, err := client.MakeResourceSpecificClient(ctx, create, restcfg)
			if err != nil {
				return err
			}

			req := client.Post()
			opts := metav1.CreateOptions{
				FieldManager: kubedef.K8sFieldManager,
			}

			if create.Namespace != "" {
				req.Namespace(create.Namespace)
			}

			r := req.Resource(resourceName(create)).
				VersionedParams(&opts, metav1.ParameterCodec).
				Body([]byte(create.BodyJson))

			if OutputKubeApiURLs {
				fmt.Fprintf(console.Debug(ctx), "kubernetes: api post call %q\n", r.URL())
			}

			if err := r.Do(ctx).Error(); err != nil {
				if errors.IsAlreadyExists(err) && create.UpdateIfExisting {
					return updateResource(ctx, d, create, restcfg)
				}

				return err
			}

			return nil
		}); err != nil {
			if !errors.IsNotFound(err) {
				return nil, fnerrors.InvocationError("%s: failed to create: %w", d.Description, err)
			}
		}

		return nil, nil
	})
}

func updateResource(ctx context.Context, d *schema.SerializedInvocation, create *kubedef.OpCreate, restcfg *rest.Config) error {
	return tasks.Action("kubernetes.update").Scope(schema.PackageNames(d.Scope...)...).
		HumanReadablef(d.Description).
		Arg("resource", resourceName(create)).
		Arg("name", create.Name).
		Arg("namespace", create.Namespace).Run(ctx, func(ctx context.Context) error {
		client, err := client.MakeResourceSpecificClient(ctx, create, restcfg)
		if err != nil {
			return err
		}

		req := client.Put()
		opts := metav1.UpdateOptions{
			FieldManager: kubedef.K8sFieldManager,
		}

		if create.Namespace != "" {
			req.Namespace(create.Namespace)
		}

		r := req.Resource(resourceName(create)).
			VersionedParams(&opts, metav1.ParameterCodec).
			Body([]byte(create.BodyJson))

		if OutputKubeApiURLs {
			fmt.Fprintf(console.Debug(ctx), "kubernetes: api put call %q\n", r.URL())
		}

		return r.Do(ctx).Error()
	})
}
