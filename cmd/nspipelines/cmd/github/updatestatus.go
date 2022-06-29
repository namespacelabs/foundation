// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package github

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"text/template"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v45/github"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/cmd/nspipelines/cmd/runs"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const pipelineLabel = "namespace-ci"

func newUpdateStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "update-status",
	}

	flag := cmd.Flags()

	// Required flags:
	installationID := flag.Int64("installation_id", -1, "Installation ID that we're requesting an access token to.")
	appID := flag.Int64("app_id", -1, "app ID of the app we're requesting an access token to.")
	privateKey := flag.String("private_key", "", "Path to the app's private key.")
	owner := flag.String("owner", "", "Organization name.")
	repo := flag.String("repo", "", "Repository name.")
	commit := flag.String("commit", "", "Commit's SHA.")

	// Optional - setting commit's status.
	phase := flag.String("phase", "", "Indicate which pipeline phase has been started. Valid values are INIT, TEST, BUILD, DEPLOY, FINAL.")
	success := flag.Bool("success", false, "Indicate the final status of the pipeline. Ignored unless --phase=FINAL.")
	specifiedUrl := flag.String("url", "", "Target URL from status entry.")
	runResult := flag.String("run_result", "", "A file with the output of runs publish.")

	// Optional - adding a comment to a commit.
	comment := flag.String("comment", "", "Comment to add to the commit.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, *appID, *installationID, *privateKey)
		if err != nil {
			return err
		}

		url := *specifiedUrl
		if *runResult != "" {
			if url != "" {
				return fnerrors.New("can't specify --url and --run_result")
			}

			var err error
			url, err = runs.MakeUrl(*runResult)
			if err != nil {
				return err
			}
		}

		client := github.NewClient(&http.Client{Transport: itr})

		current, err := parsePhase(*phase)
		if err != nil {
			return err
		}

		createStatus := func(state State, desc, label string) error {
			_, _, err := client.Repositories.CreateStatus(ctx, *owner, *repo, *commit, &github.RepoStatus{
				State:       github.String(state.String()),
				Description: github.String(desc),
				Context:     github.String(label),
				TargetURL:   github.String(url),
			})
			return err
		}

		switch current {
		case Phase_Init:
			if err := createStatus(State_Pending, "Initializing...", pipelineLabel); err != nil {
				return err
			}
		case Phase_Test:
			if err := createStatus(State_Pending, "Testing...", pipelineLabel); err != nil {
				return err
			}
			if err := createStatus(State_Pending, "", fmt.Sprintf("%s/test", pipelineLabel)); err != nil {
				return err
			}
		case Phase_Build:
			if err := createStatus(State_Pending, "Building...", pipelineLabel); err != nil {
				return err
			}
			if err := createStatus(State_Success, "", fmt.Sprintf("%s/test", pipelineLabel)); err != nil {
				return err
			}
			if err := createStatus(State_Pending, "", fmt.Sprintf("%s/build", pipelineLabel)); err != nil {
				return err
			}
		case Phase_Deploy:
			if err := createStatus(State_Pending, "Deploying...", pipelineLabel); err != nil {
				return err
			}
			if err := createStatus(State_Success, "", fmt.Sprintf("%s/test", pipelineLabel)); err != nil {
				return err
			}
			if err := createStatus(State_Success, "", fmt.Sprintf("%s/build", pipelineLabel)); err != nil {
				return err
			}
			if err := createStatus(State_Pending, "", fmt.Sprintf("%s/deploy", pipelineLabel)); err != nil {
				return err
			}
		case Phase_Final:
			if *success {
				if err := createStatus(State_Success, "Succeeded", pipelineLabel); err != nil {
					return err
				}
			} else {
				if err := createStatus(State_Failure, "Failed", pipelineLabel); err != nil {
					return err
				}
			}

			// Update pending task statuses.
			statuses, _, err := client.Repositories.ListStatuses(ctx, *owner, *repo, *commit, &github.ListOptions{})
			if err != nil {
				return err
			}
			for _, s := range statuses {
				if s.Context == nil || !strings.HasPrefix(*s.Context, fmt.Sprintf("%s/", pipelineLabel)) {
					continue
				}
				if *s.State != "pending" {
					continue
				}
				if *success {
					if err := createStatus(State_Success, "", *s.Context); err != nil {
						return err
					}
				} else {
					if err := createStatus(State_Failure, "", *s.Context); err != nil {
						return err
					}
				}
			}
		}

		if *comment != "" {
			t, err := template.New("comment").Parse(*comment)
			if err != nil {
				return fnerrors.New("failed to parse template: %w", err)
			}

			var out bytes.Buffer
			if err := t.Execute(&out, CommentsTmplData{
				URL: url,
			}); err != nil {
				return fnerrors.New("failed to render comment template: %w", err)
			}

			if _, _, err := client.Repositories.CreateComment(ctx, *owner, *repo, *commit, &github.RepositoryComment{
				Body: github.String(*comment),
			}); err != nil {
				return err
			}
		}

		return nil
	})

	return cmd
}

type CommentsTmplData struct {
	URL string
}

type Phase int

const (
	Phase_None Phase = iota
	Phase_Init
	Phase_Test
	Phase_Build
	Phase_Deploy
	Phase_Final
)

func parsePhase(phase string) (Phase, error) {
	switch phase {
	case "INIT":
		return Phase_Init, nil
	case "TEST":
		return Phase_Test, nil
	case "BUILD":
		return Phase_Build, nil
	case "DEPLOY":
		return Phase_Deploy, nil
	case "FINAL":
		return Phase_Final, nil
	default:
		return Phase_None, fmt.Errorf("Invalid phase %q", phase)
	}
}

type State int

const (
	State_Pending State = iota
	State_Success
	State_Error
	State_Failure
)

func (s State) String() string {
	switch s {
	case State_Pending:
		return "pending"
	case State_Success:
		return "success"
	case State_Error:
		return "error"
	case State_Failure:
		return "failure"
	}
	return ""
}
