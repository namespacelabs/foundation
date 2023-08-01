package cmd

import (
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/cmd/auth"
	"namespacelabs.dev/foundation/internal/cli/cmd/aws"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
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

	root.AddCommand(cluster.NewBareClusterCmd(false))
	root.AddCommand(cluster.NewKubectlCmd())    // nsc kubectl
	root.AddCommand(cluster.NewKubeconfigCmd()) // nsc kubeconfig write
	root.AddCommand(cluster.NewBuildkitCmd())   // nsc buildkit
	root.AddCommand(cluster.NewBuildCmd())      // nsc build
	root.AddCommand(cluster.NewMetadataCmd())   // nsc metadata
	root.AddCommand(cluster.NewCreateCmd())     // nsc create
	root.AddCommand(cluster.NewListCmd())       // nsc list
	root.AddCommand(cluster.NewLogsCmd())       // nsc logs
	root.AddCommand(cluster.NewExposeCmd())     // nsc expose
	root.AddCommand(cluster.NewRunCmd())        // nsc run
	root.AddCommand(cluster.NewRunComposeCmd()) // nsc run-compose
	root.AddCommand(cluster.NewSshCmd())        // nsc ssh
	root.AddCommand(cluster.NewDockerCmd())     // nsc docker
	root.AddCommand(cluster.NewDescribeCmd())   // nsc describe
	root.AddCommand(cluster.NewExecScoped())    // nsc exec-scoped
	root.AddCommand(cluster.NewIngressCmd())    // nsc ingress

	root.AddCommand(sdk.NewSdkCmd(true))

	root.AddCommand(workspace.NewWorkspaceCmd()) // nsc workspace
}
