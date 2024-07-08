// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cmd

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/admin"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/aws"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster/private"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster/terminal"
	"namespacelabs.dev/foundation/internal/cli/cmd/devbox"
	"namespacelabs.dev/foundation/internal/cli/cmd/gcp"
	"namespacelabs.dev/foundation/internal/cli/cmd/sdk"
	"namespacelabs.dev/foundation/internal/cli/cmd/version"
	"namespacelabs.dev/foundation/internal/cli/cmd/workspace"
)

func RegisterCommands(root *cobra.Command) {
	root.AddCommand(auth.NewAuthCmd())
	root.AddCommand(auth.NewLoginCmd()) // register `nsc login` as an alias for `nsc auth login`
	root.AddCommand(aws.NewAwsCmd())
	root.AddCommand(gcp.NewGcpCmd())

	root.AddCommand(version.NewVersionCmd())

	root.AddCommand(cluster.NewBareClusterCmd("instance", false))
	root.AddCommand(cluster.NewBareClusterCmd("cluster", true)) // backwards compatibility
	root.AddCommand(cluster.NewKubectlCmd())                    // nsc kubectl
	root.AddCommand(cluster.NewKubeconfigCmd())                 // nsc kubeconfig write
	root.AddCommand(cluster.NewBuildkitCmd())                   // nsc buildkit
	root.AddCommand(cluster.NewBuildCmd())                      // nsc build
	root.AddCommand(cluster.NewMetadataCmd())                   // nsc metadata
	root.AddCommand(cluster.NewCreateCmd())                     // nsc create
	root.AddCommand(cluster.NewDestroyCmd())                    // nsc destroy
	root.AddCommand(cluster.NewListCmd())                       // nsc list
	root.AddCommand(cluster.NewLogsCmd())                       // nsc logs
	root.AddCommand(cluster.NewExposeCmd())                     // nsc expose
	root.AddCommand(cluster.NewRunCmd())                        // nsc run
	root.AddCommand(cluster.NewRunComposeCmd())                 // nsc run-compose
	root.AddCommand(cluster.NewSshCmd())                        // nsc ssh
	root.AddCommand(cluster.NewVncCmd())                        // nsc vnc
	root.AddCommand(cluster.NewTopCmd())                        // nsc top
	root.AddCommand(cluster.NewDockerCmd())                     // nsc docker
	root.AddCommand(cluster.NewDescribeCmd())                   // nsc describe
	root.AddCommand(cluster.NewExecScoped())                    // nsc exec-scoped
	root.AddCommand(cluster.NewIngressCmd())                    // nsc ingress
	root.AddCommand(cluster.NewVolumeCmd())                     // nsc volume
	root.AddCommand(cluster.NewGitCheckoutCmd())                // nsc git-checkout [hidden]
	root.AddCommand(private.NewInternalCmd())                   // nsc internal [hidden]
	root.AddCommand(terminal.NewTerminalCmd())                  // nsc terminal [hidden]
	root.AddCommand(cluster.NewBazelCmd())

	root.AddCommand(sdk.NewSdkCmd(true))

	root.AddCommand(admin.NewAdminCmd(true)) // nsc admin

	root.AddCommand(workspace.NewWorkspaceCmd()) // nsc workspace

	root.AddCommand(devbox.NewDevBoxCmd()) // nsc devbox
}
