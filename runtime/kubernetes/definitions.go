// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func RegisterGraphHandlers() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.Definition, apply *kubedef.OpApply) (*ops.DispatcherResult, error) {
		if apply.Resource == "" {
			return nil, fnerrors.InternalError("%s: apply.Resource is required", d.Description)
		}

		if apply.Name == "" {
			return nil, fnerrors.InternalError("%s: apply.Name is required", d.Description)
		}

		if apply.BodyJson == "" {
			return nil, fnerrors.InternalError("%s: apply.Body is required", d.Description)
		}

		scope := asPackages(d.Scope)

		var res unstructured.Unstructured
		if err := tasks.Action("kubernetes.apply").Scope(scope...).
			HumanReadablef(d.Description).
			Arg("resource", apply.Resource).
			Arg("name", apply.Name).
			Arg("namespace", apply.Namespace).Run(ctx, func(ctx context.Context) error {
			restcfg, err := client.ResolveConfig(env)
			if err != nil {
				return err
			}

			client, err := client.MakeResourceSpecificClient(apply.Resource, restcfg)
			if err != nil {
				return err
			}

			patchOpts := kubedef.Ego().ToPatchOptions()
			req := client.Patch(types.ApplyPatchType)
			if apply.Namespace != "" {
				req = req.Namespace(apply.Namespace)
			}

			return req.Resource(apply.Resource).
				Name(apply.Name).
				VersionedParams(&patchOpts, scheme.ParameterCodec).
				Body([]byte(apply.BodyJson)).
				Do(ctx).Into(&res)
		}); err != nil {
			return nil, fnerrors.RemoteError("%s: %w", d.Description, err)
		}

		// XXX support more resource types.
		if apply.Resource == "deployments" || apply.Resource == "statefulsets" {
			generation, found1, err1 := unstructured.NestedInt64(res.Object, "metadata", "generation")
			observedGen, found2, err2 := unstructured.NestedInt64(res.Object, "status", "observedGeneration")
			if err2 != nil || !found2 {
				observedGen = 0 // Assume no generation exists.
			}

			// XXX print a warning if expected fields are missing.
			if err1 == nil && found1 {
				var waiters []ops.Waiter
				for _, sc := range scope {
					w := waitOn{
						devHost:     env.DevHost(),
						env:         env.Proto(),
						def:         d,
						apply:       apply,
						resource:    apply.Resource,
						scope:       sc,
						previousGen: observedGen,
						expectedGen: generation,
					}
					waiters = append(waiters, w)
				}
				return &ops.DispatcherResult{
					Waiters: waiters,
				}, nil
			} else {
				fmt.Fprintf(console.Warnings(ctx), "missing generation data from deployment: %v / %v [found1=%v found2=%v]\n", err1, err2, found1, found2)
			}
		}

		return nil, nil
	})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.Definition, delete *kubedef.OpDelete) (*ops.DispatcherResult, error) {
		if delete.Resource == "" {
			return nil, fnerrors.InternalError("%s: delete.Resource is required", d.Description)
		}

		if delete.Name == "" {
			return nil, fnerrors.InternalError("%s: delete.Name is required", d.Description)
		}

		if err := tasks.Action("kubernetes.delete").Scope(asPackages(d.Scope)...).
			HumanReadablef(d.Description).
			Arg("resource", delete.Resource).
			Arg("name", delete.Name).
			Arg("namespace", delete.Namespace).Run(ctx, func(ctx context.Context) error {
			restcfg, err := client.ResolveConfig(env)
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
			return nil, fnerrors.RemoteError("%s: %w", d.Description, err)
		}

		return nil, nil
	})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.Definition, deleteList *kubedef.OpDeleteList) (*ops.DispatcherResult, error) {
		if deleteList.Resource == "" {
			return nil, fnerrors.InternalError("%s: deleteList.Resource is required", d.Description)
		}

		if deleteList.Namespace == "" {
			return nil, fnerrors.InternalError("%s: deleteList.Namespace is required", d.Description)
		}

		if err := tasks.Action("kubernetes.delete-collection").Scope(asPackages(d.Scope)...).
			HumanReadablef(d.Description).
			Arg("resource", deleteList.Resource).
			Arg("selector", deleteList.LabelSelector).
			Arg("namespace", deleteList.Namespace).Run(ctx, func(ctx context.Context) error {
			restcfg, err := client.ResolveConfig(env)
			if err != nil {
				return err
			}

			client, err := client.MakeResourceSpecificClient(deleteList.Resource, restcfg)
			if err != nil {
				return err
			}

			listOpts := metav1.ListOptions{LabelSelector: deleteList.LabelSelector}

			var res unstructured.UnstructuredList
			if err := client.Get().
				Namespace(deleteList.Namespace).
				Resource(deleteList.Resource).
				VersionedParams(&listOpts, scheme.ParameterCodec).
				Do(ctx).
				Into(&res); err != nil {
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

				return client.Delete().
					Namespace(deleteList.Namespace).
					Resource(deleteList.Resource).
					Name(u.GetName()).
					Body(&opts).
					Do(ctx).Error()
			})
		}); err != nil {
			return nil, fnerrors.RemoteError("%s: %w", d.Description, err)
		}

		return nil, nil
	})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.Definition, create *kubedef.OpCreate) (*ops.DispatcherResult, error) {
		if create.Resource == "" {
			return nil, fnerrors.InternalError("%s: create.Resource is required", d.Description)
		}

		if create.Name == "" {
			return nil, fnerrors.InternalError("%s: create.Name is required", d.Description)
		}

		if create.IfMissing {
			exists, err := checkResourceExists(ctx, env, d.Description, create.Resource, create.Name, create.Namespace, asPackages(d.Scope))
			if err != nil {
				return nil, err
			}

			if exists {
				return nil, nil // Nothing to do.
			}
		}

		if err := tasks.Action("kubernetes.create").Scope(asPackages(d.Scope)...).
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
			return nil, fnerrors.RemoteError("%s: failed to create: %w", d.Description, err)
		}

		return nil, nil
	})

	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.Definition, create *kubedef.OpCreateSecretConditionally) (*ops.DispatcherResult, error) {
		wenv, ok := env.(ops.WorkspaceEnvironment)
		if !ok {
			return nil, fnerrors.InternalError("expected a ops.WorkspaceEnvironment")
		}

		if create.Name == "" {
			return nil, fnerrors.InternalError("%s: create.Name is required", d.Description)
		}

		if create.Namespace == "" {
			return nil, fnerrors.InternalError("%s: create.Namespace is required", d.Description)
		}

		exists, err := checkResourceExists(ctx, env, d.Description, "secrets", create.Name, create.Namespace, asPackages(d.Scope))
		if err != nil {
			return nil, err
		}

		if exists {
			return nil, nil // Nothing to do.
		}

		cfg, err := client.ComputeHostEnv(env.DevHost(), env.Proto())
		if err != nil {
			return nil, err
		}

		cli, err := client.NewClientFromHostEnv(cfg)
		if err != nil {
			return nil, err
		}

		invocation, err := tools.Invoke(ctx, env, wenv, schema.PackageName(create.GetInvocation().GetBinary()), false)
		if err != nil {
			return nil, err
		}

		result, err := compute.GetValue(ctx, invocation)
		if err != nil {
			return nil, err
		}

		if result.RawOutput == nil {
			return nil, fnerrors.BadInputError("%s: tool didn't produce an output", create.Invocation.Binary)
		}

		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      create.Name,
				Namespace: create.Namespace,
				Labels:    kubedef.MakeLabels(env.Proto(), nil),
			},
			Data: map[string][]byte{
				create.UserSpecifiedName: result.RawOutput,
			},
		}

		if _, err := cli.CoreV1().Secrets(create.Namespace).Create(ctx, newSecret, metav1.CreateOptions{
			FieldManager: kubedef.Ego().FieldManager,
		}); err != nil {
			return nil, err
		}

		return nil, nil
	})

	ingress.RegisterGraphHandlers()
}

func checkResourceExists(ctx context.Context, env ops.Environment, description, resource, name, namespace string, scope []schema.PackageName) (bool, error) {
	var exists bool
	// XXX this is racy here, we need to have a loop and a callback for contents.
	if err := tasks.Action("kubernetes.get").Scope(scope...).
		HumanReadablef("Check: "+description).
		Arg("resource", resource).
		Arg("name", name).
		Arg("namespace", namespace).Run(ctx, func(ctx context.Context) error {
		restcfg, err := client.ResolveConfig(env)
		if err != nil {
			return err
		}

		client, err := client.MakeResourceSpecificClient(resource, restcfg)
		if err != nil {
			return err
		}

		opts := metav1.GetOptions{}
		req := client.Get()
		if namespace != "" {
			req.Namespace(namespace)
		}

		if err := req.Resource(resource).
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

func asPackages(input []string) []schema.PackageName {
	var scope []schema.PackageName
	for _, s := range input {
		scope = append(scope, schema.PackageName(s))
	}
	return scope
}
