// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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

	phase := flag.String("phase", "", "Indicate which pipeline phase has been started. Valid values are INIT, TEST, BUILD, DEPLOY, FINAL.")
	success := flag.Bool("success", false, "Indicate the final status of the pipeline. Ignored unless --phase=FINAL.")
	specifiedUrl := flag.String("url", "", "Target URL from status entry.")
	runResult := flag.String("run_result", "", "A file with the output of runs publish.")
	pipelineState := flag.String("pipeline_state", "", "A file the pipeline state.")

	// Optional - adding a comment to a commit.
	comment := flag.String("comment", "", "Comment to add to the commit.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		current, err := parsePhase(*phase)
		if err != nil {
			return err
		}

		var state PipelineState
		if current != Phase_Init {
			if err := readState(*pipelineState, &state); err != nil {
				return fnerrors.New("unable to read pipeline state: %w", err)
			}
		}

		if err := updateState(current, *success, *pipelineState, &state); err != nil {
			return nil
		}

		if serialized, err := json.MarshalIndent(state, "", " "); err == nil {
			fmt.Fprintf(os.Stdout, "Updated pipeline state:\n%s\n", string(serialized))
		}

		itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, *appID, *installationID, *privateKey)
		if err != nil {
			return err
		}
		client := github.NewClient(&http.Client{Transport: itr})

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

		if err := publishState(ctx, client, *owner, *repo, *commit, url, state); err != nil {
			return nil
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
				Body: github.String(out.String()),
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

func (p Phase) Context() string {
	switch p {
	case Phase_Init:
		return pipelineLabel
	case Phase_Test:
		return fmt.Sprintf("%s/test", pipelineLabel)
	case Phase_Build:
		return fmt.Sprintf("%s/build", pipelineLabel)
	case Phase_Deploy:
		return fmt.Sprintf("%s/deploy", pipelineLabel)
	}
	return ""
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

type PipelinePhase struct {
	Phase Phase `json:"phase"`
	State State `json:"state"`
}

type PipelineState struct {
	Desc   string           `json:"desc"`
	Phases []*PipelinePhase `json:"phases"`
}

func readState(file string, out *PipelineState) error {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return fnerrors.New("unable to read pipeline state: %w", err)
	}

	if err := json.Unmarshal(content, out); err != nil {
		return fnerrors.New("unable to parse pipeline state: %w", err)
	}

	return nil
}

func updateState(current Phase, success bool, file string, state *PipelineState) error {
	switch current {
	case Phase_Init:
		state.Desc = "Initializing..."
	case Phase_Test:
		state.Desc = "Testing..."
	case Phase_Build:
		state.Desc = "Building..."
		for _, p := range state.Phases {
			if p.Phase == Phase_Test {
				p.State = State_Success
			}
		}
	case Phase_Deploy:
		state.Desc = "Deploying..."
		for _, p := range state.Phases {
			if p.Phase == Phase_Build {
				p.State = State_Success
			}
		}
	case Phase_Final:
		state.Desc = ""
		final := State_Success
		if !success {
			final = State_Failure
		}

		for _, p := range state.Phases {
			if p.State == State_Pending {
				p.State = final
			}
		}
	}

	if current != Phase_Final {
		state.Phases = append(state.Phases, &PipelinePhase{
			Phase: current,
			State: State_Pending,
		})
	}

	serialized, err := json.MarshalIndent(*state, "", " ")
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(file, serialized, 0644); err != nil {
		return fnerrors.New("failed to write %q: %w", file, err)
	}

	return nil
}

func publishState(ctx context.Context, client *github.Client, owner, repo, commit, url string, state PipelineState) error {
	for _, p := range state.Phases {
		desc := ""
		if p.Phase == Phase_Init {
			desc = state.Desc
		}

		if _, _, err := client.Repositories.CreateStatus(ctx, owner, repo, commit, &github.RepoStatus{
			State:       github.String(p.State.String()),
			Description: github.String(desc),
			Context:     github.String(p.Phase.Context()),
			TargetURL:   github.String(url),
		}); err != nil {
			return err
		}

	}
	return nil
}
