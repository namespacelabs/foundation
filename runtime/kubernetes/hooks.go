// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	kubenode "namespacelabs.dev/foundation/std/runtime/kubernetes"
)

func prepareApplyServerExtensions(ctx context.Context, env ops.Environment, srv *schema.Server) (*frontend.PrepareProps, error) {
	var ensureServiceAccount bool

	if err := configure.VisitAllocs(srv.Allocation, kubeNode, &kubenode.ServerExtensionArgs{},
		func(instance *schema.Allocation_Instance, instantiate *schema.Instantiate, args *kubenode.ServerExtensionArgs) error {
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

	serviceAccount := kubedef.MakeDeploymentId(srv)

	saDetails := &kubedef.ServiceAccountDetails{ServiceAccountName: serviceAccount}
	packedSaDetails, err := anypb.New(saDetails)
	if err != nil {
		return nil, err
	}

	packedExt, err := anypb.New(&kubedef.SpecExtension{
		EnsureServiceAccount: true,
		ServiceAccount:       serviceAccount,
	})
	if err != nil {
		return nil, err
	}

	return &frontend.PrepareProps{
		ProvisionInput: []*anypb.Any{packedSaDetails},
		Extension: []*schema.DefExtension{{
			For:  srv.PackageName,
			Impl: packedExt,
		}},
	}, nil
}
