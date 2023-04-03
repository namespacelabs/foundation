// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package metadata

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	defaultMetadataSpecFile = "/var/run/nsc/token.spec"
)

var supportedMetadataKeys = []string{"id"}

func NewMetadataCmd(hidden bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "metadata",
		Short:  "Work with VM metadata.",
		Hidden: hidden,
	}

	cmd.AddCommand(newReadCmd())

	return cmd
}

func newReadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read [metadata-key]",
		Short: "Read a metadata value for the provided key.",
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, args []string) error {
		metadataKey := args[0]
		if !slices.Contains(supportedMetadataKeys, metadataKey) {
			return fnerrors.New("reading cluster metadata value for the key %q is not supported", metadataKey)
		}

		var specStr string
		if s, ok := os.LookupEnv("NSC_METADATA_SPEC"); ok {
			specStr = s
		} else {
			specFile := defaultMetadataSpecFile
			if f, ok := os.LookupEnv("NSC_METADATA_SPEC_FILE"); ok {
				specFile = f
			}

			s, err := os.ReadFile(specFile)
			if err != nil {
				return fnerrors.New("failed to read metadata spec file: %w", err)
			}

			specStr = string(s)
		}

		specData, err := base64.RawStdEncoding.DecodeString(specStr)
		if err != nil {
			fmt.Fprintf(console.Debug(ctx), "failed to base64 decode metadata spec: %v", err)
			return fnerrors.New("metadata spec is not base64 encoded")
		}

		var spec MetadataSpec
		if err := json.Unmarshal(specData, &spec); err != nil {
			fmt.Fprintf(console.Debug(ctx), "failed to unmarshal metadata spec: %v", err)
			return fnerrors.New("metadata spec is invalid")
		}

		value, err := readMetadataValue(ctx, spec, metadataKey)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stdout(ctx), value+"\n")

		return nil
	})

	return cmd
}

type MetadataSpec struct {
	Version     string `json:"version,omitempty"`
	MetadataURL string `json:"metadata_url,omitempty"`
}

func readMetadataValue(ctx context.Context, spec MetadataSpec, key string) (string, error) {
	switch spec.Version {
	case "v1":
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(tCtx, http.MethodGet, fmt.Sprintf("%s/%s", spec.MetadataURL, key), nil)
		if err != nil {
			return "", fnerrors.New("failed to create metadata request: %w", err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return "", fnerrors.New("failed to fetch metadata value: %w", err)
		}

		defer resp.Body.Close()

		valueBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fnerrors.New("failed to read metadata value: %w", err)
		}
		return string(valueBytes), nil

	default:
		return "", fnerrors.New("metadata spec is not supported; only support version=v1")
	}
}
