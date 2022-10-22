// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	"namespacelabs.dev/foundation/internal/git"
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
	commandLine := flags.String("command_line", "", "Command to reproduce the run.")
	workspaceDir := flags.String("workspace", ".", "The workspace directory to parse.")
	pipelineName := flags.String("pipeline_name", "", "Name of the pipeline that spawned this run (e.g. autopush, preview).")
	nspipelinesVersion := flags.String("nspipelines_version", "", "Digest of nspipelines image.")
	kind := flags.String("invocation_kind", "", "If set, adds an InvocationDescription to the run.")

	// At most one of these should be set.
	githubEvent := flags.String("github_event_path", "", "Path to a file with github's event json.")
	author := flags.String("author_login", "", "Path to a file with github's event json.")

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
				Kind:        storage.InvocationDescription_Kind(parsedKind),
				CommandLine: *commandLine,
			})
		}

		if *githubEvent != "" {
			if *author != "" {
				return fnerrors.BadInputError("can't specify --github_event_path and --author_login")
			}

			if *workspaceDir == "" {
				return fnerrors.BadInputError("--workspace is required")
			}

			workspaceData, err := cuefrontend.ModuleLoader.ModuleAt(ctx, *workspaceDir)
			if err != nil {
				return err
			}

			eventData, err := os.ReadFile(*githubEvent)
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
				AuthorLogin:        ev.Sender.Login,
				ModuleName:         []string{workspaceData.ModuleName()},
				PipelineName:       *pipelineName,
				NspipelinesVersion: *nspipelinesVersion,
			})
		} else if *parentRunIDPath == "" {
			// This code path is executed when running `ns starter` in a pipeline.
			// Even though there is no Github event, we still need to fill "storage.RunMetadata".

			workspaceData, err := cuefrontend.ModuleLoader.ModuleAt(ctx, *workspaceDir)
			var moduleName string
			// The workspace may be not initialized yet, for example before running `ns starter`.
			if err == nil {
				moduleName = workspaceData.ModuleName()
			}

			remoteUrl, err := git.RemoteUrl(ctx, *workspaceDir)
			if err != nil {
				return err
			}

			status, err := git.FetchStatus(ctx, *workspaceDir)
			if err != nil {
				return err
			}

			branch, err := git.CurrentBranch(ctx, *workspaceDir)
			if err != nil {
				return err
			}

			attachments = append(attachments, &storage.RunMetadata{
				Branch:             branch,
				Repository:         remoteUrl,
				CommitId:           status.Revision,
				AuthorLogin:        *author,
				ModuleName:         []string{moduleName},
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
		if err := fnapi.AnonymousCall(ctx, storageEndpoint, fmt.Sprintf("%s/NewRun", storageService), req, fnapi.DecodeJSONResponse(&resp)); err != nil {
			return err
		}

		if resp.RunId == "" {
			return fnerrors.InvocationError("expected a run id to be produced")
		}

		r, err := json.Marshal(resp)
		if err != nil {
			return fnerrors.InternalError("failed to marshal response: %w", err)
		}

		fmt.Fprintf(os.Stdout, "Created run %q, parent: %q\n", resp.RunId, req.ParentRunId)
		fmt.Fprintln(os.Stdout, "Attachments:")
		for _, attachment := range attachments {
			text, _ := json.MarshalIndent(attachment, "", "  ")
			fmt.Fprintf(os.Stdout, "%s\n", text)
		}

		return os.WriteFile(*storeRunID, r, 0644)
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
