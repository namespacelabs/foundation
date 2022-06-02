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
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

func RegisterCreateSecret() {
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, d *schema.Definition, create *kubedef.OpCreateSecretConditionally) (*ops.HandleResult, error) {
		wenv, ok := env.(workspace.WorkspaceEnvironment)
		if !ok {
			return nil, fnerrors.InternalError("expected a workspace.WorkspaceEnvironment")
		}

		if create.Name == "" {
			return nil, fnerrors.InternalError("%s: create.Name is required", d.Description)
		}

		if create.Namespace == "" {
			return nil, fnerrors.InternalError("%s: create.Namespace is required", d.Description)
		}

		exists, err := checkResourceExists(ctx, env, d.Description, "secrets", create.Name, create.Namespace, schema.PackageNames(d.Scope...))
		if err != nil {
			return nil, err
		}

		if exists {
			return nil, nil // Nothing to do.
		}

		cli, err := client.NewClient(client.ConfigFromEnv(ctx, env))
		if err != nil {
			return nil, err
		}

		invocation, err := tools.Invoke(ctx, env, wenv, create.GetInvocation())
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
}
