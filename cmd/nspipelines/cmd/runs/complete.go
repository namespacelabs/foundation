// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"bytes"
	"context"
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

const (
	// Section log data will be streamed in chunks of this size.
	// The chunks need to fit into Envoy transfer buffer (1M by default).
	uploadChunkSize = 128 * 1024
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

			var reqs = []*UploadSectionRunRequest{}
			var bytes = out.Bytes()
			for i := 0; i < len(bytes); i += uploadChunkSize {
				chunkEnd := i + uploadChunkSize
				if chunkEnd > len(bytes) {
					chunkEnd = len(bytes)
				}
				chunk := bytes[i:chunkEnd]
				req := &UploadSectionRunRequest{
					Payload: chunk,
				}
				if i == 0 {
					req.OpaqueUserAuth = userAuth.Opaque
					req.RunId = run.RunId
					req.PayloadFormat = "application/vnd.namespace.run+pb-zstd"
					req.PayloadLength = len(bytes)
				}
				reqs = append(reqs, req)
			}

			if err := fnapi.AnonymousCall(ctx, storageEndpoint, fmt.Sprintf("%s/UploadSectionStream", storageService), reqs, nil); err != nil {
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

		if err := fnapi.AnonymousCall(ctx, storageEndpoint, fmt.Sprintf("%s/CompleteRun", storageService),
			&CompleteRunRequest{OpaqueUserAuth: userAuth.Opaque, RunId: runID}, nil); err != nil {
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
	PayloadLength  int    `json:"payload_length,omitempty"`
	Payload        []byte `json:"payload,omitempty"`
}

type CompleteRunRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	RunId          string `json:"run_id,omitempty"`
}
