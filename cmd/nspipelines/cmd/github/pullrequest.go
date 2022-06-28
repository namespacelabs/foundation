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
	"github.com/google/go-github/v45/github"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func newPullRequestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "pull-request",
	}

	flag := cmd.Flags()

	// Required flags:
	installationID := flag.Int64("installation_id", -1, "Installation ID that we're requesting an access token to.")
	appID := flag.Int64("app_id", -1, "app ID of the app we're requesting an access token to.")
	privateKey := flag.String("private_key", "", "Path to the app's private key.")
	owner := flag.String("owner", "", "Organization name.")
	repo := flag.String("repo", "", "Repository name.")
	branch := flag.String("branch", "", "For which Github branch shall we list open pull requests.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, *appID, *installationID, *privateKey)
		if err != nil {
			return err
		}

		client := github.NewClient(&http.Client{Transport: itr})

		prs, _, err := client.PullRequests.List(ctx, *owner, *repo, &github.PullRequestListOptions{})
		if err != nil {
			return err
		}

		hasPullRequest := false
		for _, pr := range prs {
			if *pr.Head.Ref == *branch {
				hasPullRequest = true
			}
		}

		fmt.Fprintf(os.Stdout, "%v", hasPullRequest)

		return nil
	})
	return cmd
}
