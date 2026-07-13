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
	"os/exec"
	"time"

	bazelv1betagrpc "buf.build/gen/go/namespace/cloud/grpc/go/proto/namespace/cloud/integrations/bazel/v1beta/bazelv1betagrpc"
	bazelv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/bazel/v1beta"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/grpc"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/nsc/grpcapi"
)

const (
	bazelExecutionPathBase = "bazelrbe"
	defaultBazelRbeCommand = "build"
)

func newSetupExecutionCmd() *cobra.Command {
	return newSetupExecutionCmdWithRemoteFlag(false)
}

func newSetupBazelCmd() *cobra.Command {
	return newSetupExecutionCmdWithRemoteFlag(true)
}

func newSetupExecutionCmdWithRemoteFlag(includeRemoteFlag bool) *cobra.Command {
	var bazelRcPath, output, bazelCommand, key string
	var staticDur time.Duration
	var static, enableRemoteAssetAPI bool
	remote := true

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
		if includeRemoteFlag {
			flags.BoolVar(&remote, "remote", true, "If false, configure remote caching and build events without remote execution.")
		}
	}).Do(func(ctx context.Context) error {
		tok, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		authMode := bazelv1beta.BazelExecutionAuthMode_BAZEL_EXECUTION_AUTH_MODE_MTLS
		if static {
			authMode = bazelv1beta.BazelExecutionAuthMode_BAZEL_EXECUTION_AUTH_MODE_STATIC
		}

		res, err := ensureBazelExecutionCluster(ctx, tok, key, authMode, enableRemoteAssetAPI)
		if err != nil {
			return fnerrors.Newf("failed to provision bazel execution cluster: %w", err)
		}

		if res.GetSchedulerEndpoint() == "" || res.GetStorageEndpoint() == "" {
			return fnerrors.Newf("received incomplete response (scheduler=%q storage=%q)", res.GetSchedulerEndpoint(), res.GetStorageEndpoint())
		}

		out := bazelRbeSetup{
			SchedulerEndpoint:     res.GetSchedulerEndpoint(),
			StorageEndpoint:       res.GetStorageEndpoint(),
			RemoteAssetEndpoint:   res.GetRemoteAssetEndpoint(),
			Jobs:                  int32(res.GetRecommendedBazelJobs()),
			RemoteTimeout:         time.Duration(res.GetRecommendedBazelRemoteTimeoutSeconds()) * time.Second,
			RemoteLocalFallback:   res.GetRecommendedBazelRemoteLocalFallback(),
			RemoteDownloadOutputs: res.GetRecommendedBazelRemoteDownloadOutputs(),
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
		out.BuildEventEndpoint = res.GetBuildEventEndpoint()
		out.CredentialHelperDomains = res.GetCredentialHelperDomains()

		if bazelRcPath != "" {
			data, err := toBazelExecutionConfig(ctx, out, bazelCommand, remote)
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
				data, err := toBazelExecutionConfig(ctx, out, bazelCommand, remote)
				if err != nil {
					return err
				}

				loc, err := writeTempFile(bazelExecutionPathBase, "*.bazelrc", data)
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				bazelRcPath = loc
			}

			configuration := "remote execution"
			if !remote {
				configuration = "remote caching without remote execution"
			}
			fmt.Fprintf(console.Stdout(ctx), "Wrote bazelrc configuration for %s to %s.\n", configuration, bazelRcPath)

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

func toBazelExecutionConfig(ctx context.Context, out bazelRbeSetup, command string, remote bool) ([]byte, error) {
	var buf bytes.Buffer

	var lines []string
	if remote {
		lines = append(lines, fmt.Sprintf("--remote_executor=%s", out.SchedulerEndpoint))
	}
	lines = append(lines, fmt.Sprintf("--remote_cache=%s", out.StorageEndpoint))
	if remote {
		lines = append(lines,
			"--spawn_strategy=remote",
			"--genrule_strategy=remote",
			fmt.Sprintf("--remote_local_fallback=%t", out.RemoteLocalFallback),
		)
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

func ensureBazelExecutionCluster(ctx context.Context, tok *auth.Token, key string, authMode bazelv1beta.BazelExecutionAuthMode, enableRemoteAssetAPI bool) (*bazelv1beta.EnsureClusterResponse, error) {
	client, conn, err := newBazelServiceClient(ctx, tok)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	req := &bazelv1beta.EnsureClusterRequest{}
	req.SetKey(key)
	req.SetAuthMode(authMode)
	req.SetEnableRemoteAssetApi(enableRemoteAssetAPI)

	return client.EnsureCluster(ctx, req)
}

func newBazelServiceClient(ctx context.Context, tok *auth.Token) (bazelv1betagrpc.BazelServiceClient, *grpc.ClientConn, error) {
	conn, err := grpcapi.NewConnectionWithEndpoint(ctx, fnapi.GlobalEndpoint(), tok)
	if err != nil {
		return nil, nil, err
	}

	return bazelv1betagrpc.NewBazelServiceClient(conn), conn, nil
}
