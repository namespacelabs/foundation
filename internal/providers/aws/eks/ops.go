// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package eks

import (
	"context"

	awsauth "github.com/keikoproj/aws-auth/pkg/mapper"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

func RegisterGraphHandlers() {
	execution.RegisterFuncs(execution.Funcs[*OpEnsureAwsAuth]{
		Handle: func(ctx context.Context, def *schema.SerializedInvocation, a *OpEnsureAwsAuth) (*execution.HandleResult, error) {
			cluster, err := kubedef.InjectedKubeCluster(ctx)
			if err != nil {
				return nil, err
			}

			awsAuth := awsauth.New(cluster.PreparedClient().Clientset, false)
			args := &awsauth.MapperArguments{
				MapRoles: true,
				RoleARN:  a.Rolearn,
				Username: a.Username,
				Groups:   a.Group,
			}

			if err := awsAuth.Upsert(args); err != nil {
				return nil, fnerrors.Newf("unable to update AWS auth configmap: %w", err)
			}

			return nil, nil
		},
	})
}
