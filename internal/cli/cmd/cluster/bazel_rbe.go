// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	bazelExecutionPathBase = "bazelrbe"
	defaultBazelRbeCommand = "build"

	// TODO: replace with a generated public-proto Connect client
	bazelExecutionEnsureProcedure = "/namespace.private.bazel.BazelService/EnsureBazelExecutionCluster"
)

func newSetupExecutionCmd() *cobra.Command {
	var bazelRcPath, output, bazelCommand, key string

	return fncobra.Cmd(&cobra.Command{
		Use:    "setup",
		Short:  "Set up a remote Bazel execution cluster and generate a bazelrc to use it.",
		Hidden: true,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&bazelRcPath, "bazelrc", "", "If specified, write the bazelrc to this path.")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.StringVar(&bazelCommand, "command", defaultBazelRbeCommand, "The bazel command to use in the generated bazelrc (e.g., 'build' or 'common').")
		flags.StringVar(&key, "key", "", "Stable identifier that disambiguates multiple parallel execution clusters for the same workspace. Defaults to 'default'.")
	}).Do(func(ctx context.Context) error {
		tok, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		endpoints, err := ensureBazelExecutionCluster(ctx, tok, key)
		if err != nil {
			return fnerrors.Newf("failed to provision bazel execution cluster: %w", err)
		}

		if endpoints.SchedulerEndpoint == "" || endpoints.StorageEndpoint == "" {
			return fnerrors.Newf("received incomplete response (scheduler=%q storage=%q)", endpoints.SchedulerEndpoint, endpoints.StorageEndpoint)
		}

		privateKeyPem, publicKeyPem, err := genPrivAndPublicKeysPEM()
		if err != nil {
			return fnerrors.Newf("failed to generate client key pair: %w", err)
		}

		privateKeyPem, err = convertECPrivateKeyToPKCS8(privateKeyPem)
		if err != nil {
			return fnerrors.Newf("failed to encode client key in PKCS#8: %w", err)
		}

		clientCertPem, err := fetchClientCert(ctx, string(publicKeyPem))
		if err != nil {
			return fnerrors.Newf("failed to issue client certificate: %w", err)
		}

		clientCertPath, err := writeTempFile(bazelExecutionPathBase, "*.cert", []byte(clientCertPem))
		if err != nil {
			return fnerrors.Newf("failed to create client cert temp file: %w", err)
		}

		clientKeyPath, err := writeTempFile(bazelExecutionPathBase, "*.key", privateKeyPem)
		if err != nil {
			return fnerrors.Newf("failed to create client key temp file: %w", err)
		}

		out := bazelRbeSetup{
			SchedulerEndpoint:     endpoints.SchedulerEndpoint,
			StorageEndpoint:       endpoints.StorageEndpoint,
			ClientCert:            clientCertPath,
			ClientKey:             clientKeyPath,
			Jobs:                  int32(endpoints.Jobs),
			RemoteTimeout:         time.Duration(endpoints.RemoteTimeoutSeconds) * time.Second,
			RemoteLocalFallback:   endpoints.RemoteLocalFallback,
			RemoteDownloadOutputs: endpoints.RemoteDownloadOutputs,
		}

		if bazelRcPath != "" {
			data, err := toBazelExecutionConfig(out, bazelCommand)
			if err != nil {
				return err
			}

			if err := writeFile(bazelRcPath, data); err != nil {
				return err
			}
		}

		switch output {
		case "json":
			d := json.NewEncoder(console.Stdout(ctx))
			d.SetIndent("", "  ")
			if err := d.Encode(out); err != nil {
				return fnerrors.InternalError("failed to encode response as JSON: %w", err)
			}

		default:
			if output != "" && output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
			}

			if bazelRcPath == "" {
				data, err := toBazelExecutionConfig(out, bazelCommand)
				if err != nil {
					return err
				}

				loc, err := writeTempFile(bazelExecutionPathBase, "*.bazelrc", data)
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				bazelRcPath = loc
			}

			fmt.Fprintf(console.Stdout(ctx), "Wrote bazelrc configuration for remote execution to %s.\n", bazelRcPath)

			style := colors.Ctx(ctx)
			fmt.Fprintf(console.Stdout(ctx), "\nStart using it by adding:\n")
			fmt.Fprintf(console.Stdout(ctx), "  %s", style.Highlight.Apply(fmt.Sprintf("--bazelrc=%s\n", bazelRcPath)))
		}

		return nil
	})
}

type bazelRbeSetup struct {
	SchedulerEndpoint     string        `json:"scheduler_endpoint,omitempty"`
	StorageEndpoint       string        `json:"storage_endpoint,omitempty"`
	ClientCert            string        `json:"client_cert,omitempty"`
	ClientKey             string        `json:"client_key,omitempty"`
	Jobs                  int32         `json:"jobs,omitempty"`
	RemoteTimeout         time.Duration `json:"remote_timeout,omitempty"`
	RemoteLocalFallback   bool          `json:"remote_local_fallback,omitempty"`
	RemoteDownloadOutputs string        `json:"remote_download_outputs,omitempty"`
}

func toBazelExecutionConfig(out bazelRbeSetup, command string) ([]byte, error) {
	var buf bytes.Buffer

	lines := []string{
		fmt.Sprintf("--remote_executor=%s", out.SchedulerEndpoint),
		fmt.Sprintf("--remote_cache=%s", out.StorageEndpoint),
		"--spawn_strategy=remote",
		"--genrule_strategy=remote",
		fmt.Sprintf("--remote_local_fallback=%t", out.RemoteLocalFallback),
	}

	if out.ClientCert != "" {
		lines = append(lines, fmt.Sprintf("--tls_client_certificate=%s", out.ClientCert))
	}
	if out.ClientKey != "" {
		lines = append(lines, fmt.Sprintf("--tls_client_key=%s", out.ClientKey))
	}

	if out.RemoteDownloadOutputs != "" {
		lines = append(lines, fmt.Sprintf("--remote_download_outputs=%s", out.RemoteDownloadOutputs))
	}
	if out.Jobs > 0 {
		lines = append(lines, fmt.Sprintf("--jobs=%d", out.Jobs))
	}
	if out.RemoteTimeout > 0 {
		// Bazel expects whole seconds.
		lines = append(lines, fmt.Sprintf("--remote_timeout=%d", int(out.RemoteTimeout.Seconds())))
	}

	for _, line := range lines {
		if _, err := fmt.Fprintf(&buf, "%s %s\n", command, line); err != nil {
			return nil, fnerrors.Newf("failed to write bazelrc line: %w", err)
		}
	}

	return buf.Bytes(), nil
}

type ensureBazelExecutionClusterRequest struct {
	Key string `json:"key,omitempty"`
}

// Fields mirror EnsureBazelExecutionClusterResponse in
// private/proto/service/bazel/service.proto. The server marshals with
// protojson (UseProtoNames=true), so the wire tags are snake_case.
type ensureBazelExecutionClusterResponse struct {
	SchedulerEndpoint     string `json:"scheduler_endpoint,omitempty"`
	StorageEndpoint       string `json:"storage_endpoint,omitempty"`
	Jobs                  uint32 `json:"jobs,omitempty"`
	RemoteTimeoutSeconds  uint32 `json:"remote_timeout_seconds,omitempty"`
	RemoteLocalFallback   bool   `json:"remote_local_fallback,omitempty"`
	RemoteDownloadOutputs string `json:"remote_download_outputs,omitempty"`
}

// ensureBazelExecutionCluster posts to the private BazelService path on the
// global API endpoint using the connect-json protocol. Once the public proto
// has the same RPC, replace this with a generated Connect client (mirroring
// fnapi.NewBazelCacheServiceClient).
func ensureBazelExecutionCluster(ctx context.Context, tok *auth.Token, key string) (*ensureBazelExecutionClusterResponse, error) {
	bearer, err := tok.IssueToken(ctx, 5*time.Minute, false)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(ensureBazelExecutionClusterRequest{Key: key})
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(fnapi.GlobalEndpoint(), "/") + bazelExecutionEnsureProcedure
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+bearer)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fnerrors.Newf("EnsureBazelExecutionCluster returned %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	out := &ensureBazelExecutionClusterResponse{}
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, fnerrors.Newf("failed to parse response: %w", err)
	}

	return out, nil
}
