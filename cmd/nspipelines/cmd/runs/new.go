// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/cmd/nspipelines/cmd/runs/github"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/cli/version"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/schema/storage"
)

const storageEndpoint = "https://grpc-gateway-eg999pfts0vbcol25ao0.prod.namespacelabs.nscloud.dev"
const storageService = "nsl.runs.storage.RunStorageService"

func newNewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "new",
		Args: cobra.NoArgs,
	}

	flags := cmd.Flags()

	storeRunID := flags.String("output_run_id_path", "", "Where to output the run id.")
	parentRunIDPath := flags.String("parent_run_id_path", "", "The parent run id.")
	workspaceDir := flags.String("workspace", ".", "The workspace directory to parse.")
	githubEvent := flags.String("github_event_path", "", "Path to a file with github's event json.")
	pipelineName := flags.String("pipeline_name", "", "Name of the pipeline that spawned this run (e.g. autopush, preview).")
	nspipelinesVersion := flags.String("nspipelines_version", "", "Digest of nspipelines image.")
	kind := flags.String("invocation_kind", "", "If set, adds an InvocationDescription to the run.")

	_ = cmd.MarkFlagRequired("output_run_id_path")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		userAuth, err := fnapi.LoadUser()
		if err != nil {
			return err
		}

		var attachments []proto.Message

		if v, err := version.Current(); err == nil {
			attachments = append(attachments, v)
		} else {
			log.Printf("won't attach versioning information: %v", err)
		}

		if *kind != "" {
			parsedKind, ok := storage.InvocationDescription_Kind_value[strings.ToUpper(*kind)]
			if !ok {
				return fnerrors.BadInputError("%s: no such kind", *kind)
			}

			attachments = append(attachments, &storage.InvocationDescription{
				Kind: storage.InvocationDescription_Kind(parsedKind),
				// XXX command line
			})
		}

		if *githubEvent != "" {
			if *workspaceDir == "" {
				return fnerrors.BadInputError("--workspace is required")
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

			attachments = append(attachments, &storage.GithubEvent{SerializedJson: string(eventData)})
			attachments = append(attachments, &storage.RunMetadata{
				Branch:             parseBranch(ev.Ref),
				Repository:         "github.com/" + ev.Repository.FullName,
				CommitId:           ev.HeadCommit.ID,
				ModuleName:         []string{workspaceData.Parsed().ModuleName},
				PipelineName:       *pipelineName,
				NspipelinesVersion: *nspipelinesVersion,
			})
		}

		req := &NewRunRequest{
			OpaqueUserAuth: userAuth.Opaque,
		}

		if *parentRunIDPath != "" {
			r, err := storedrun.LoadStoredID(*parentRunIDPath)
			if err != nil {
				return err
			}

			req.ParentRunId = r.RunId
		}

		for _, attachment := range attachments {
			any, err := anypb.New(attachment)
			if err != nil {
				return fnerrors.InternalError("failed to serialize attachment: %w", err)
			}
			// req.Attachment = append(req.Attachment, any)
			req.ManualAttachment = append(req.ManualAttachment, &NewRunRequest_Attachment{
				TypeUrl:     any.TypeUrl,
				Base64Value: base64.RawStdEncoding.EncodeToString(any.Value),
			})
		}

		var resp storedrun.StoredRunID
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
	OpaqueUserAuth   []byte                      `json:"opaque_user_auth,omitempty"`
	ParentRunId      string                      `json:"parent_run_id,omitempty"`
	Attachment       []*anypb.Any                `json:"attachment,omitempty"`
	ManualAttachment []*NewRunRequest_Attachment `json:"manual_attachment,omitempty"`
}

type NewRunRequest_Attachment struct {
	TypeUrl     string `json:"type_url,omitempty"`
	Base64Value string `json:"base64_value,omitempty"`
}
