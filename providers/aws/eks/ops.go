// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"context"

	awsauth "github.com/keikoproj/aws-auth/pkg/mapper"
	"k8s.io/client-go/kubernetes"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
)

func RegisterGraphHandlers() {
	ops.RegisterFunc(func(ctx context.Context, env planning.Context, def *schema.SerializedInvocation, a *OpEnsureAwsAuth) (*ops.HandleResult, error) {
		restcfg, err := client.ResolveConfig(ctx, env)
		if err != nil {
			return nil, err
		}
		clientset, err := kubernetes.NewForConfig(restcfg)
		if err != nil {
			return nil, err
		}
		awsAuth := awsauth.New(clientset, false)
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
