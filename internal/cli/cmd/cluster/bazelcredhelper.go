// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
)

func NewBazelCredHelperGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "get",
		Short:  "Supply a credential for a remote server to bazel.",
		Args:   cobra.NoArgs,
		Hidden: false,
	}

	cmd.RunE = fncobra.RunE(func(ctx context.Context, _ []string) error {
		return bazelCredGet(ctx)
	})

	return cmd
}

// Input JSON for "get".
// See https://github.com/bazelbuild/proposals/blob/main/designs/2022-06-07-bazel-credential-helpers.md#proposal
// and https://github.com/EngFlow/credential-helper-spec/blob/main/schemas/get-credentials-request.schema.json
type getInput struct {
	Uri string `json:"uri"`
}

// Header name to (potentially multiple) values
type headers map[string][]string

// Output JSON for "get".
// See https://github.com/EngFlow/credential-helper-spec/blob/main/schemas/get-credentials-response.schema.json
type getOutput struct {
	Expires string  `json:"expires,omitempty"` // Should be RFC 3339 according to spec, but bazel doesn't like nanoseconds.
	Headers headers `json:"headers"`
}

func bazelCredGet(ctx context.Context) error {
	done := console.EnterInputMode(ctx)
	defer done()

	input, err := readStdin()
	if err != nil {
		return fnerrors.New("failed to read from stdin: %w", err)
	}

	var req getInput
	if err := json.Unmarshal(input, &req); err != nil {
		return fnerrors.New("failed to parse JSON from stdin: %w", err)
	}

	url, err := url.Parse(req.Uri)
	if err != nil {
		return fnerrors.New("failed to parse '%s' as URL: %w", req.Uri, err)
	}

	hdrs, expires, err := fetchHeaders(ctx, url)
	if err != nil {
		return err
	}

	resp := getOutput{
		Expires: maybeFormatTime(expires),
		Headers: hdrs,
	}
	output, err := json.Marshal(resp)
	if err != nil {
		return fnerrors.New("failed to marshal JSON: %w", err)
	}

	n, err := os.Stdout.Write(output)
	if err != nil {
		return fnerrors.New("failed to output to stdout: %w", err)
	}
	if n != len(output) {
		return fnerrors.New("failed to write %d bytes to stdout, only wrote %d", len(output), n)
	}

	return nil
}

// Returns:
// - auth headers to set
// - until when bazel is allowed to cache those
func fetchHeaders(ctx context.Context, url *url.URL) (headers, *time.Time, error) {
	if url.Scheme != "https" {
		return nil, nil, fnerrors.New("nsc bazel credential helper configured for non-https endpoint")
	}

	// Bazel asking for credentials is a good time to ensure that the remote bazel cache is initialized.
	response, err := api.EnsureBazelCache(ctx, api.Methods, "workstation")
	if err != nil {
		return nil, nil, err
	}

	cacheUrl, err := url.Parse(response.HttpsCacheEndpoint)
	if err != nil {
		return nil, nil, fnerrors.New("internal error: bad URL from EnsureBazelCache: %v", err)
	}

	if cacheUrl.Host != url.Host {
		return nil, nil, fnerrors.New("Please re-configure --remote-cache=%s", cacheUrl.Host)
	}

	token, err := fnapi.IssueToken(ctx, 1*time.Hour)
	if err != nil {
		return nil, nil, err
	}

	// Ask a bit more often than token expiration to limit the impact in case the instance has an issue.
	expires := time.Now().Add(15 * time.Minute)

	return headers{
		"Authorization": []string{"Bearer " + token},
	}, &expires, nil
}

func maybeFormatTime(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339)
}
