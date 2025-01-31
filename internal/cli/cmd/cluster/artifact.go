package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewArtifactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "artifact",
		Short:  "Artifact-related activities.",
		Hidden: true,
	}

	cmd.AddCommand(newArtifactUploadCmd())

	return cmd
}

func newArtifactUploadCmd() *cobra.Command {
	var targetPath, namespace, localPath string
	var expiresIn time.Duration

	return fncobra.Cmd(&cobra.Command{
		Use:   "upload",
		Short: "Upload an artifact.",
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&targetPath, "target-path", "", "Target path of the artifact.")
		flags.StringVar(&namespace, "namespace", "", "Target namespace of the artifact.")
		flags.DurationVar(&expiresIn, "expires-in", 7*24*time.Hour, "When to expire the artifact.")
		flags.StringVar(&localPath, "local-path", "", "Local path of the artifact.")
	}).Do(func(ctx context.Context) error {
		expiresAt := time.Now().Add(expiresIn)

		res, err := api.CreateArtifact(ctx, api.Methods, targetPath, namespace, expiresAt)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), "Upload url: %q", res.SignedUploadUrl)

		// XXX read local file and upload.

		return nil
	})
}
