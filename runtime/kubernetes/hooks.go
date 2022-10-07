// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/allocations"
	"namespacelabs.dev/foundation/std/planning"
	kubenode "namespacelabs.dev/foundation/std/runtime/kubernetes"
)

func prepareApplyServerExtensions(_ context.Context, _ planning.Context, srv *schema.Stack_Entry) (*frontend.InternalPrepareProps, error) {
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

	saDetails := &kubedef.ServiceAccountDetails{ServiceAccountName: serviceAccount}

	packedExt, err := anypb.New(&kubedef.SpecExtension{
		EnsureServiceAccount: true,
		ServiceAccount:       serviceAccount,
	})
	if err != nil {
		return nil, err
	}

	var props frontend.InternalPrepareProps
	props.ProvisionInput = []rtypes.ProvisionInput{
		{Message: saDetails},
	}
	props.Extension = []*schema.DefExtension{{
		For:  srv.Server.PackageName,
		Impl: packedExt,
	}}
	return &props, nil
}
