// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/storedrun"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func newCompleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "complete",
		Args: cobra.NoArgs,
	}

	flags := cmd.Flags()

	runIDPath := flags.String("run_id_path", "", "The run id.")
	storedRun := flags.String("stored_run_path", "", "Path to a file with a stored run's contents.")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		userAuth, err := fnapi.LoadUser()
		if err != nil {
			return err
		}

		var runID string
		if *storedRun != "" {
			run, marshalled, err := protos.ReadFileAndBytes[*storage.UndifferentiatedRun](*storedRun)
			if err != nil {
				return fnerrors.BadInputError("invalid run: %w", err)
			}

			if run.RunId == "" {
				return fnerrors.BadInputError("missing embedded run id")
			}

			runID = run.RunId

			var out bytes.Buffer
			enc, err := zstd.NewWriter(&out)
			if err != nil {
				return err
			}

			if _, err := enc.Write(marshalled); err != nil {
				return fnerrors.InternalError("failed to compress: %w", err)
			}

			if err := enc.Close(); err != nil {
				return fnerrors.InternalError("failed to complete compression: %w", err)
			}

			req := &UploadSectionRunRequest{
				OpaqueUserAuth: userAuth.Opaque,
				RunId:          run.RunId,
				PayloadFormat:  "application/vnd.namespace.run+pb-zstd",
				Payload:        out.Bytes(),
			}

			if err := fnapi.CallAPI(ctx, storageEndpoint, fmt.Sprintf("%s/UploadSection", storageService), req, func(dec *json.Decoder) error {
				// No response to check.
				return nil
			}); err != nil {
				return err
			}
		}

		if *runIDPath != "" {
			r, err := storedrun.LoadStoredID(*runIDPath)
			if err != nil {
				return err
			}

			if runID != "" && runID != r.RunId {
				return fnerrors.BadInputError("inconsistent run ids %q vs %q", runID, r.RunId)
			}

			runID = r.RunId
		}

		if runID == "" {
			return fnerrors.BadInputError("either --run_id_path or --stored_run_path are required")
		}

		if err := fnapi.CallAPI(ctx, storageEndpoint, fmt.Sprintf("%s/CompleteRun", storageService),
			&CompleteRunRequest{OpaqueUserAuth: userAuth.Opaque, RunId: runID},
			func(dec *json.Decoder) error {
				// No response to check.
				return nil
			}); err != nil {
			return err
		}

		fmt.Fprintf(os.Stdout, "Completed run %q\n", runID)

		return nil
	})

	return cmd
}

type UploadSectionRunRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	RunId          string `json:"run_id,omitempty"`
	PayloadFormat  string `json:"payload_format,omitempty"`
	Payload        []byte `json:"payload,omitempty"`
}

type CompleteRunRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	RunId          string `json:"run_id,omitempty"`
}
