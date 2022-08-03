// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func newAccessTokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "access-token",
		Args: cobra.NoArgs,
	}

	flag := cmd.Flags()

	session := flag.String("session", "", "Session ID for a custom Namespace pipeline run.")

	// The following flags may only be set if session is not set.
	installationID := flag.Int64("installation_id", -1, "Installation ID that we're requesting an access token to.")
	appID := flag.Int64("app_id", -1, "app ID of the app we're requesting an access token to.")
	privateKey := flag.String("private_key", "", "Path to the app's private key.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		var token string
		if *session != "" {
			if *appID != -1 || *installationID != -1 || *privateKey != "" {
				return fmt.Errorf("invalid flag setting: if --session is set, --installation_id, --app_id and --private_key may not be set.")
			}

			var err error
			token, err = getSessionToken(ctx, *session)
			if err != nil {
				return err
			}
		} else {
			itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, *appID, *installationID, *privateKey)
			if err != nil {
				return err
			}

			token, err = itr.Token(ctx)
			if err != nil {
				return err
			}
		}

		fmt.Fprintln(os.Stdout, token)
		return nil
	})

	return cmd
}

const WorkspaceService = "nsl.workspace.WorkspaceService"

type GetGithubTokenRequest struct {
	SessionId string `json:"session_id"`
}
type GetGithubTokenResponse struct {
	Token string `json:"token"`
}

func getSessionToken(ctx context.Context, session string) (string, error) {
	req := &GetGithubTokenRequest{
		SessionId: session,
	}

	var resp GetGithubTokenResponse
	if err := fnapi.CallAPI(ctx, fnapi.EndpointAddress, fmt.Sprintf("%s/GetGithubToken", WorkspaceService), req, func(dec *json.Decoder) error {
		if err := dec.Decode(&resp); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return "", err
	}

	return resp.Token, nil
}
