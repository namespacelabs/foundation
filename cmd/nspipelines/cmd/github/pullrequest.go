// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
	output := flag.String("output", "", "Where to write whether a pull request has been found.")

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

		var relatedPrs []*github.PullRequest
		for _, pr := range prs {
			if *pr.State == "closed" {
				continue
			}
			if *pr.Head.Ref != *branch {
				continue
			}

			relatedPrs = append(relatedPrs, pr)
		}

		fmt.Fprintf(os.Stdout, "Found %d open pull requests related to branch %s\n", len(relatedPrs), *branch)
		for _, pr := range relatedPrs {
			fmt.Fprintf(os.Stdout, " - %s at %s (%s)\n", *pr.Title, *pr.HTMLURL, *pr.State)
		}

		hasPr := len(relatedPrs) > 0
		return os.WriteFile(*output, []byte(fmt.Sprintf("%v", hasPr)), 0644)
	})
	return cmd
}
