// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/types"
)

func RegisterCreateSecret() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.SerializedInvocation, create *kubedef.OpCreateSecretConditionally) (*ops.HandleResult, error) {
		if create.Name == "" {
			return nil, fnerrors.InternalError("%s: create.Name is required", d.Description)
		}

		if create.Namespace == "" {
			return nil, fnerrors.InternalError("%s: create.Namespace is required", d.Description)
		}

		v, err := ops.Value[*types.DeferredResource](d, "value")
		if err != nil {
			return nil, fnerrors.New("%s: failed to retrieve value: %w", d.Description, err)
		}

		exists, err := checkResourceExists(ctx, env, d.Description, "secrets", create.Name, create.Namespace, schema.PackageNames(d.Scope...))
		if err != nil {
			return nil, err
		}

		if exists {
			return nil, nil // Nothing to do.
		}

		resource, err := ResolveResource(ctx, env, v)
		if err != nil {
			return nil, fnerrors.New("%s: failed to retrieve value: %w", d.Description, err)
		}

		if resource.GetContents() == nil {
			return nil, fnerrors.BadInputError("%s: resource is missing a value", d.Description)
		}

		cli, err := client.NewClient(client.ConfigFromEnv(ctx, env))
		if err != nil {
			return nil, err
		}

		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      create.Name,
				Namespace: create.Namespace,
				Labels:    kubedef.MakeLabels(env.Proto(), nil),
			},
			Data: map[string][]byte{
				create.UserSpecifiedName: resource.GetContents(),
			},
		}

		if _, err := cli.CoreV1().Secrets(create.Namespace).Create(ctx, newSecret, metav1.CreateOptions{
			FieldManager: kubedef.Ego().FieldManager,
		}); err != nil {
			return nil, err
		}

		return nil, nil
	})
}
