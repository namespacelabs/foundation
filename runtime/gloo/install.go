// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package gloo

import (
	"context"

	"github.com/solo-io/gloo/projects/gloo/cli/pkg/cmd/install"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

// Install sets up the gloo gateway on Kubernetes.
func Install(ctx context.Context) error {
	helmClient := install.DefaultHelmClient()
	installer := install.NewInstaller(helmClient)
	mode := install.Gloo
	if err := installer.Install(&install.InstallerConfig{
		Mode: mode,
		Ctx:  ctx,
	}); err != nil {
		return fnerrors.InternalError("failed to install gloo: %w", err)
	}
	return nil
}
