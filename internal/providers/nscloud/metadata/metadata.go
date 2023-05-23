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
	"time"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	tokenMetadataKey = "workload_token"
)

type MetadataSpec struct {
	Version     string `json:"version,omitempty"`
	MetadataURL string `json:"metadata_url,omitempty"`
	TokenURL    string `json:"token_url,omitempty"`
	InlineToken string `json:"inline_token,omitempty"`
}

func FetchValueFromSpec(ctx context.Context, specData string, key string) (string, error) {
	decodedSpec, err := base64.RawStdEncoding.DecodeString(specData)
	if err != nil {
		return "", fnerrors.New("metadata spec is invalid")
	}

	var spec MetadataSpec
	if err := json.Unmarshal(decodedSpec, &spec); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to unmarshal metadata spec: %v", err)
		return "", fnerrors.New("metadata spec is invalid")
	}

	if spec.InlineToken != "" {
		return spec.InlineToken, nil
	}

	switch spec.Version {
	case "v1":
		mdURL := fmt.Sprintf("%s/%s", spec.MetadataURL, key)
		tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(tCtx, http.MethodGet, mdURL, nil)
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

func FetchTokenFromSpec(ctx context.Context, specData string) (string, error) {
	return FetchValueFromSpec(ctx, specData, tokenMetadataKey)
}
