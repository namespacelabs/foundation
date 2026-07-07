// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
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
	var staticDur time.Duration
	var static, enableRemoteAssetAPI bool

	return fncobra.Cmd(&cobra.Command{
		Use:    "setup",
		Short:  "Set up a remote Bazel execution cluster and generate a bazelrc to use it.",
		Hidden: true,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&bazelRcPath, "bazelrc", "", "If specified, write the bazelrc to this path.")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.StringVar(&bazelCommand, "command", defaultBazelRbeCommand, "The bazel command to use in the generated bazelrc (e.g., 'build' or 'common').")
		flags.StringVar(&key, "key", "", "Stable identifier that disambiguates multiple parallel execution clusters for the same workspace. Defaults to 'default'.")
		flags.BoolVar(&static, "static", false, "If specified, authenticate using a static bearer token in --remote_header against the public endpoints instead of issuing an mTLS client certificate.")
		flags.BoolVar(&enableRemoteAssetAPI, "enable_remote_asset_api", false, "If specified, opt-in to the remote asset API and configure bazel's --experimental_remote_downloader.")
		fncobra.DurationVar(flags, &staticDur, "static_token_duration", 4*time.Hour, "The minimum duration of the static token configured (requires --static).")
	}).Do(func(ctx context.Context) error {
		tok, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		authMode := bazelExecutionAuthModeMTLS
		if static {
			authMode = bazelExecutionAuthModeStatic
		}

		res, err := ensureBazelExecutionCluster(ctx, tok, key, authMode, enableRemoteAssetAPI)
		if err != nil {
			return fnerrors.Newf("failed to provision bazel execution cluster: %w", err)
		}

		if res.SchedulerEndpoint == "" || res.StorageEndpoint == "" {
			return fnerrors.Newf("received incomplete response (scheduler=%q storage=%q)", res.SchedulerEndpoint, res.StorageEndpoint)
		}

		out := bazelRbeSetup{
			SchedulerEndpoint:     res.SchedulerEndpoint,
			StorageEndpoint:       res.StorageEndpoint,
			RemoteAssetEndpoint:   res.RemoteAssetEndpoint,
			Jobs:                  int32(res.Jobs),
			RemoteTimeout:         time.Duration(res.RemoteTimeoutSeconds) * time.Second,
			RemoteLocalFallback:   res.RemoteLocalFallback,
			RemoteDownloadOutputs: res.RemoteDownloadOutputs,
		}

		if static {
			// In static mode the server returns the public, bearer-authenticated
			// endpoints (and deliberately does not expose the mTLS endpoints). A
			// bearer token (e.g. a revocable token) can be revoked server-side,
			// whereas a client certificate minted from it cannot be easily
			// revoked, so we authenticate with the token itself via the ingress
			// auth header instead of issuing a client certificate.
			bearerToken, err := tok.IssueToken(ctx, staticDur, false)
			if err != nil {
				return fnerrors.Newf("failed to issue bearer token: %w", err)
			}

			out.IngressAuthToken = bearerToken
		} else {
			privateKeyPem, publicKeyPem, err := genPrivAndPublicKeysPEM()
			if err != nil {
				return fnerrors.Newf("failed to generate client key pair: %w", err)
			}

			privateKeyPem, err = convertECPrivateKeyToPKCS8(privateKeyPem)
			if err != nil {
				return fnerrors.Newf("failed to encode client key in PKCS#8: %w", err)
			}

			clientCertPem, err := fetchTenantClientCert(ctx, string(publicKeyPem))
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

			out.ClientCert = clientCertPath
			out.ClientKey = clientKeyPath
		}

		// The build event endpoint is a separate, bearer-authenticated host
		// (the regional API gateway), independent of the scheduler/storage auth
		// mode. The server only returns it (and the credential helper domains
		// used to authenticate it in the default mTLS mode) when build event
		// ingestion is enabled for the workspace.
		out.BuildEventEndpoint = res.BuildEventEndpoint
		out.CredentialHelperDomains = res.CredentialHelperDomains

		if bazelRcPath != "" {
			data, err := toBazelExecutionConfig(ctx, out, bazelCommand)
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
				data, err := toBazelExecutionConfig(ctx, out, bazelCommand)
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
	SchedulerEndpoint       string        `json:"scheduler_endpoint,omitempty"`
	StorageEndpoint         string        `json:"storage_endpoint,omitempty"`
	RemoteAssetEndpoint     string        `json:"remote_asset_endpoint,omitempty"`
	ClientCert              string        `json:"client_cert,omitempty"`
	ClientKey               string        `json:"client_key,omitempty"`
	IngressAuthToken        string        `json:"ingress_auth_token,omitempty"`
	Jobs                    int32         `json:"jobs,omitempty"`
	RemoteTimeout           time.Duration `json:"remote_timeout,omitempty"`
	RemoteLocalFallback     bool          `json:"remote_local_fallback,omitempty"`
	RemoteDownloadOutputs   string        `json:"remote_download_outputs,omitempty"`
	BuildEventEndpoint      string        `json:"build_event_endpoint,omitempty"`
	CredentialHelperDomains []string      `json:"credential_helper_domains,omitempty"`
}

func toBazelExecutionConfig(ctx context.Context, out bazelRbeSetup, command string) ([]byte, error) {
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
	if out.IngressAuthToken != "" {
		lines = append(lines, fmt.Sprintf("--remote_header=x-nsc-ingress-auth=Bearer\\ %s", out.IngressAuthToken))
	}

	if out.RemoteAssetEndpoint != "" {
		lines = append(lines, fmt.Sprintf("--experimental_remote_downloader=%s", out.RemoteAssetEndpoint))
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

	if out.BuildEventEndpoint != "" {
		lines = append(lines, fmt.Sprintf("--bes_backend=%s", out.BuildEventEndpoint))

		if out.IngressAuthToken != "" {
			lines = append(lines,
				fmt.Sprintf("--bes_header=Authorization=Bearer\\ %s", out.IngressAuthToken),
				fmt.Sprintf("--bes_header=x-nsc-ingress-auth=Bearer\\ %s", out.IngressAuthToken),
			)
		}
	}

	for _, line := range lines {
		if _, err := fmt.Fprintf(&buf, "%s %s\n", command, line); err != nil {
			return nil, fnerrors.Newf("failed to write bazelrc line: %w", err)
		}
	}

	if out.BuildEventEndpoint != "" && out.IngressAuthToken == "" && len(out.CredentialHelperDomains) > 0 {
		if err := appendExecutionBuildEventCredentialHelper(ctx, &buf, command, out.CredentialHelperDomains); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func appendExecutionBuildEventCredentialHelper(ctx context.Context, buf *bytes.Buffer, command string, domains []string) error {
	if _, err := exec.LookPath(BazelCredHelperBinary); err != nil {
		stdout := console.Stdout(ctx)
		style := colors.Ctx(ctx)

		if errors.Is(err, exec.ErrNotFound) {
			fmt.Fprintln(stdout)
			fmt.Fprint(stdout, style.Highlight.Apply(fmt.Sprintf("We didn't find %s in your $PATH.", BazelCredHelperBinary)))
			fmt.Fprintf(stdout, "\nIt's usually installed along-side nsc; so if you have added nsc to the $PATH, %s will also be available.\n", BazelCredHelperBinary)
			fmt.Fprintf(stdout, "\nWhile your $PATH is not updated, sending build events won't work.\n")
		}
		if !errors.Is(err, exec.ErrNotFound) {
			return fnerrors.Newf("failed to look up %s in $PATH: %w", BazelCredHelperBinary, err)
		}
	}

	for _, domain := range domains {
		if _, err := fmt.Fprintf(buf, "%s --credential_helper=*.%s=%s\n", command, domain, BazelCredHelperBinary); err != nil {
			return fnerrors.Newf("failed to append credential_helper: %w", err)
		}
	}

	return nil
}

// BazelExecutionAuthMode mirrors the enum in
// private/proto/service/bazel/service.proto. The server determines which
// endpoints to return based on the requested mode.
const (
	bazelExecutionAuthModeMTLS   = "BAZEL_EXECUTION_AUTH_MODE_MTLS"
	bazelExecutionAuthModeStatic = "BAZEL_EXECUTION_AUTH_MODE_STATIC"
)

type ensureBazelExecutionClusterRequest struct {
	Key string `json:"key,omitempty"`
	// AuthMode tells the server which authentication mode the client intends to
	// use; it returns the matching endpoints. Static mode returns the public
	// bearer-authenticated endpoints and never exposes the mTLS endpoints.
	AuthMode string `json:"auth_mode,omitempty"`
	// EnableRemoteAssetApi opts into the remote asset API; when set the server
	// returns remote_asset_endpoint in the response.
	EnableRemoteAssetApi bool `json:"enable_remote_asset_api,omitempty"`
}

// Fields mirror EnsureBazelExecutionClusterResponse in
// private/proto/service/bazel/service.proto. The server marshals with
// protojson (UseProtoNames=true), so the wire tags are snake_case.
type ensureBazelExecutionClusterResponse struct {
	SchedulerEndpoint       string   `json:"scheduler_endpoint,omitempty"`
	StorageEndpoint         string   `json:"storage_endpoint,omitempty"`
	RemoteAssetEndpoint     string   `json:"remote_asset_endpoint,omitempty"`
	Jobs                    uint32   `json:"jobs,omitempty"`
	RemoteTimeoutSeconds    uint32   `json:"remote_timeout_seconds,omitempty"`
	RemoteLocalFallback     bool     `json:"remote_local_fallback,omitempty"`
	RemoteDownloadOutputs   string   `json:"remote_download_outputs,omitempty"`
	BuildEventEndpoint      string   `json:"build_event_endpoint,omitempty"`
	CredentialHelperDomains []string `json:"credential_helper_domains,omitempty"`
}

// ensureBazelExecutionCluster posts to the private BazelService path on the
// global API endpoint using the connect-json protocol. Once the public proto
// has the same RPC, replace this with a generated Connect client (mirroring
// fnapi.NewBazelCacheServiceClient).
func ensureBazelExecutionCluster(ctx context.Context, tok *auth.Token, key, authMode string, enableRemoteAssetAPI bool) (*ensureBazelExecutionClusterResponse, error) {
	bearer, err := tok.IssueToken(ctx, 5*time.Minute, false)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(ensureBazelExecutionClusterRequest{Key: key, AuthMode: authMode, EnableRemoteAssetApi: enableRemoteAssetAPI})
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
