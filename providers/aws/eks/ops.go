// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"

	awsauth "github.com/keikoproj/aws-auth/pkg/mapper"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

func RegisterGraphHandlers() {
	ops.RegisterFunc(func(ctx context.Context, def *schema.SerializedInvocation, a *OpEnsureAwsAuth) (*ops.HandleResult, error) {
		cluster, err := kubedef.InjectedKubeCluster(ctx)
		if err != nil {
			return nil, err
		}

		awsAuth := awsauth.New(cluster.Client(), false)
		args := &awsauth.MapperArguments{
			MapRoles: true,
			RoleARN:  a.Rolearn,
			Username: a.Username,
			Groups:   a.Group,
		}

		if err := awsAuth.Upsert(args); err != nil {
			return nil, fnerrors.New("unable to update AWS auth configmap: %w", err)
		}

		return nil, nil
	})
}
