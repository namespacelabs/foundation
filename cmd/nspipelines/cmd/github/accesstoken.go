// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package github

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newAccessTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "access-token",
		Args: cobra.NoArgs,
	}

	flag := cmd.Flags()

	installationID := flag.Int64("installation_id", -1, "Installation ID that we're requesting an access token to.")
	appID := flag.Int64("app_id", -1, "app ID of the app we're requesting an access token to.")
	privateKey := flag.String("private_key", "", "Path to the app's private key.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, *appID, *installationID, *privateKey)
		if err != nil {
			return err
		}

		token, err := itr.Token(ctx)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, token)
		return nil
	})

	return cmd
}
