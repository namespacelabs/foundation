// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/klauspost/compress/zstd"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema/storage"
	"namespacelabs.dev/foundation/workspace/source/protos"
)

func newCompleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "complete",
		Args: cobra.NoArgs,
	}

	flags := cmd.Flags()

	storedRun := flags.String("stored_run_path", "", "Path to a file with a stored run's contents.")
	_ = cmd.MarkFlagRequired("stored_run_path")

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		userAuth, err := fnapi.LoadUser()
		if err != nil {
			return err
		}

		run, marshalled, err := protos.ReadFileAndBytes[*storage.UndifferentiatedRun](*storedRun)
		if err != nil {
			return fnerrors.BadInputError("invalid run: %w", err)
		}

		if run.RunId == "" {
			return fnerrors.BadInputError("missing embedded run id")
		}

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

		if err := fnapi.CallAPI(ctx, storageEndpoint, fmt.Sprintf("%s/NewRun", storageService), req, func(dec *json.Decoder) error {
			// No response to check.
			return nil
		}); err != nil {
			return err
		}

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
