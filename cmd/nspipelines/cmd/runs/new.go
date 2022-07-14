// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/cmd/nspipelines/cmd/runs/github"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
)

const storageEndpoint = "https://grpc-gateway-eg999pfts0vbcol25ao0.prod.namespacelabs.nscloud.dev"
const storageService = "nsl.runs.storage.RunStorageService"

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "new",
		Args: cobra.NoArgs,
	}

	flags := cmd.Flags()

	storeRunID := flags.String("output_run_id", "", "Where to output the run id.")
	parentRunID := flags.String("parent_run_id", "", "The parent run id.")
	workspaceDir := flags.String("workspace", ".", "The workspace directory to parse.")
	githubEvent := flags.String("github_event_path", "", "Path to a file with github's event json.")

	cmd.MarkFlagRequired("output_run_id")
	cmd.MarkFlagRequired("workspace")
	cmd.MarkFlagRequired("github_event_path")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		userAuth, err := fnapi.LoadUser()
		if err != nil {
			return err
		}

		workspaceData, err := cuefrontend.ModuleLoader.ModuleAt(ctx, *workspaceDir)
		if err != nil {
			return err
		}

		eventData, err := ioutil.ReadFile(*githubEvent)
		if err != nil {
			return err
		}

		var ev github.PushEvent
		if err := json.Unmarshal(eventData, &ev); err != nil {
			return fnerrors.BadInputError("failed to unmarshal push event: %w", err)
		}

		req := &NewRunRequest{
			OpaqueUserAuth: userAuth.Opaque,
			ParentRunId:    *parentRunID,
			Metadata: &RunMetadata{
				Branch:              parseBranch(ev.Ref),
				Repository:          "github.com/" + ev.Repository.FullName,
				CommitId:            ev.HeadCommit.ID,
				ModuleName:          []string{workspaceData.Parsed().ModuleName},
				GithubEventMetadata: string(eventData),
			},
		}

		var resp NewRunResponse
		if err := fnapi.CallAPI(ctx, storageEndpoint, fmt.Sprintf("%s/NewRun", storageService), req, func(dec *json.Decoder) error {
			if err := dec.Decode(&resp); err != nil {
				return err
			}

			if resp.RunId == "" {
				return fnerrors.InvocationError("expected a run id to be produced")
			}

			return nil
		}); err != nil {
			return err
		}

		r, err := json.Marshal(resp)
		if err != nil {
			return fnerrors.InternalError("failed to marshal response: %w", err)
		}

		return ioutil.WriteFile(*storeRunID, r, 0644)
	})

	return cmd
}

func parseBranch(ref string) string {
	if strings.HasPrefix(ref, "refs/heads/") {
		return strings.TrimPrefix(ref, "refs/heads/")
	}

	return ""
}

type NewRunRequest struct {
	OpaqueUserAuth []byte       `json:"opaque_user_auth,omitempty"`
	ParentRunId    string       `json:"parent_run_id,omitempty"`
	Metadata       *RunMetadata `json:"metadata,omitempty"`
}

type RunMetadata struct {
	Repository          string   `json:"repository,omitempty"`
	Branch              string   `json:"branch,omitempty"`
	CommitId            string   `json:"commit_id,omitempty"`
	ModuleName          []string `json:"module_name,omitempty"`
	GithubEventMetadata string   `json:"github_event_metadata,omitempty"`
}

type NewRunResponse struct {
	RunId string `json:"run_id,omitempty"`
}
