// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/planning/planninghooks"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/allocations"
	"namespacelabs.dev/foundation/std/cfg"
	kubenode "namespacelabs.dev/foundation/std/runtime/kubernetes"
)

func prepareApplyServerExtensions(_ context.Context, _ cfg.Context, srv *schema.Stack_Entry) (*planninghooks.InternalPrepareProps, error) {
	var ensureServiceAccount bool

	if err := allocations.Visit(srv.Server.Allocation, kubeNode, &kubenode.ServerExtensionArgs{},
		func(_ *schema.Allocation_Instance, _ *schema.Instantiate, args *kubenode.ServerExtensionArgs) error {
			if args.EnsureServiceAccount {
				ensureServiceAccount = true
			}
			return nil
		}); err != nil {
		return nil, err
	}

	if !ensureServiceAccount {
		return nil, nil
	}

	serviceAccount := kubedef.MakeDeploymentId(srv.Server)

	packedExt, err := anypb.New(&kubedef.SpecExtension{
		EnsureServiceAccount: true,
		ServiceAccount:       serviceAccount,
	})
	if err != nil {
		return nil, err
	}

	var props planninghooks.InternalPrepareProps
	props.ProvisionInput = []rtypes.ProvisionInput{
		{Message: &kubedef.ServiceAccountDetails{ServiceAccountName: serviceAccount}},
	}
	props.Extension = []*schema.DefExtension{{
		Impl: packedExt,
	}}
	return &props, nil
}
