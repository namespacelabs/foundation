// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	bazelv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/bazel/v1beta"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/integrations/api"
)

const (
	buck2IngressAuthHeader = "x-nsc-ingress-auth"

	defaultBuck2TokenDuration = 4 * time.Hour

	// Every generated buckconfig uses this name so that a second setup run
	// replaces the first. Two differently named files would both be read, and
	// buck2 would merge them key by key into a mix of two clusters.
	buck2ConfigFileName = "50-namespace"

	buck2ConfigDirName = ".buckconfig.d"
)

func NewBuck2Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buck2",
		Short: "Buck2-related activities.",
	}

	cache := &cobra.Command{Use: "cache", Short: "Buck2 cache related functionality."}
	cache.AddCommand(newSetupBuck2CacheCmd())

	cmd.AddCommand(cache)
	cmd.AddCommand(newSetupBuck2ExecutionCmd())

	return cmd
}

// NewBuck2CacheCmd returns a "buck2" command with setup directly underneath,
// for use under "nsc cache buck2 setup".
func NewBuck2CacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buck2",
		Short: "Buck2 cache related functionality.",
	}

	cmd.AddCommand(newSetupBuck2CacheCmd())

	return cmd
}

func newSetupBuck2CacheCmd() *cobra.Command {
	var buckConfigPath, output, tokenFile, instanceName string
	var staticDur time.Duration

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote Buck2 cache and generate a buckconfig to use it.",
		Long: `Set up a remote Buck2 cache and generate a buckconfig to use it.

Writes $HOME/.buckconfig.d/50-namespace by default, which applies to every
buck2 project for your user. Pass --buckconfig to scope it to one project,
e.g. --buckconfig=<project>/.buckconfig.d/50-namespace.

buck2 builds its remote execution configuration when the daemon starts, and
only from its on-disk config files, so there is no equivalent of bazel's
--bazelrc: a file passed with --config-file is ignored for this purpose, and a
running daemon keeps its old configuration until 'buck2 killall'.

buck2 cannot invoke a credential helper, so the generated buckconfig embeds a
bearer token that expires. Re-run this command to refresh it.`,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&buckConfigPath, "buckconfig", "", "Write the buckconfig to this path instead of $HOME/.buckconfig.d/50-namespace.")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.StringVar(&tokenFile, "token", "", "Use the bearer token stored at this location for authentication instead of the default.")
		flags.StringVar(&instanceName, "instance_name", "", "If specified, set [buck2_re_client].instance_name.")
		fncobra.DurationVar(flags, &staticDur, "static_token_duration", defaultBuck2TokenDuration, "The minimum duration of the token embedded in the buckconfig.")
	}).Do(func(ctx context.Context) error {
		tok, err := buck2TokenSource(ctx, tokenFile)
		if err != nil {
			return err
		}

		client := fnapi.NewBazelCacheServiceClientWithToken(tok)

		resp, err := retryBazelProvisioning(ctx, func() (*connect.Response[bazelv1beta.EnsureBazelCacheResponse], error) {
			return client.EnsureBazelCache(ctx, connect.NewRequest(makeEnsureBazelCacheRequest(1, false, false, "")))
		})
		if err != nil {
			return fnerrors.Newf("failed to provision bazel cache: %w", err)
		}

		// buck2 authenticates with a bearer token, which only the public
		// ingress endpoint accepts; the mTLS-only cache endpoint would reject it.
		if resp.Msg.GetHttpsCacheEndpoint() == "" {
			return fnerrors.Newf("did not receive a bearer-authenticated cache endpoint")
		}

		out, err := buck2CacheOnlySetup(resp.Msg.GetHttpsCacheEndpoint(), instanceName)
		if err != nil {
			return err
		}

		if err := issueBuck2Token(ctx, tok, staticDur, &out); err != nil {
			return err
		}

		return emitBuck2Config(ctx, out, buckConfigPath, output, "remote cache")
	})
}

func newSetupBuck2ExecutionCmd() *cobra.Command {
	var buckConfigPath, output, key, tokenFile, instanceName string
	var storageMode string
	var staticDur time.Duration
	remote := true

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote Buck2 execution cluster and generate a buckconfig to use it.",
		Long: `Set up a remote Buck2 execution cluster and generate a buckconfig to use it.

Writes $HOME/.buckconfig.d/50-namespace by default, which applies to every
buck2 project for your user. Pass --buckconfig to scope it to one project,
e.g. --buckconfig=<project>/.buckconfig.d/50-namespace.

buck2 builds its remote execution configuration when the daemon starts, and
only from its on-disk config files, so there is no equivalent of bazel's
--bazelrc: a file passed with --config-file is ignored for this purpose, and a
running daemon keeps its old configuration until 'buck2 killall'.

This only configures how buck2 reaches Namespace. Running actions remotely also
requires an execution platform that sets remote_enabled; see
https://buck2.build/docs/users/remote_execution/ Such a platform races local
against remote unless local execution is disabled, so cheap actions still tend
to run locally; build with --remote-only to check remote execution itself.

buck2 cannot invoke a credential helper, so the generated buckconfig embeds a
bearer token that expires. Re-run this command to refresh it.`,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&buckConfigPath, "buckconfig", "", "Write the buckconfig to this path instead of $HOME/.buckconfig.d/50-namespace.")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.StringVar(&key, "key", "", "Stable identifier that disambiguates multiple parallel execution clusters for the same workspace. Defaults to 'default'.")
		flags.StringVar(&tokenFile, "token", "", "Use the bearer token stored at this location for authentication instead of the default.")
		flags.StringVar(&instanceName, "instance_name", "", "If specified, set [buck2_re_client].instance_name.")
		flags.BoolVar(&remote, "remote", true, "If false, configure remote caching without remote execution.")
		flags.StringVar(&storageMode, "storage", bazelStorageReadWrite, "Storage access mode. Valid options: read-only, read-write.")
		fncobra.DurationVar(flags, &staticDur, "static_token_duration", defaultBuck2TokenDuration, "The minimum duration of the token embedded in the buckconfig.")
	}).Do(func(ctx context.Context) error {
		if storageMode != bazelStorageReadOnly && storageMode != bazelStorageReadWrite {
			return fnerrors.BadInputError("invalid storage mode %q (valid values: read-only, read-write)", storageMode)
		}
		if remote && storageMode == bazelStorageReadOnly {
			return fnerrors.BadInputError("--storage=read-only requires --remote=false")
		}

		tok, err := buck2TokenSource(ctx, tokenFile)
		if err != nil {
			return err
		}

		// buck2 has no credential helper, so we always ask for the static auth
		// mode, which makes the server hand back the public,
		// bearer-authenticated endpoints instead of the mTLS ones.
		authMode := bazelv1beta.BazelExecutionAuthMode_BAZEL_EXECUTION_AUTH_MODE_STATIC

		var out buck2Setup
		if remote {
			res, err := ensureBazelExecutionCluster(ctx, tok, key, authMode, false)
			if err != nil {
				return fnerrors.Newf("failed to provision bazel execution cluster: %w", err)
			}

			if res.GetSchedulerEndpoint() == "" || res.GetStorageEndpoint() == "" {
				return fnerrors.Newf("received incomplete response (scheduler=%q storage=%q)", res.GetSchedulerEndpoint(), res.GetStorageEndpoint())
			}

			out, err = buck2ExecutionSetup(res.GetSchedulerEndpoint(), res.GetStorageEndpoint(), instanceName)
			if err != nil {
				return err
			}

			out.GrpcTimeout = time.Duration(res.GetRecommendedBazelRemoteTimeoutSeconds()) * time.Second
		} else {
			res, err := ensureBazelStorageCluster(ctx, tok, key, authMode, false, bazelStorageAccessMode(storageMode))
			if err != nil {
				return fnerrors.Newf("failed to provision bazel storage cluster: %w", err)
			}

			if res.GetStorageEndpoint() == "" {
				return fnerrors.Newf("received incomplete response (storage=%q)", res.GetStorageEndpoint())
			}

			out, err = buck2CacheOnlySetup(res.GetStorageEndpoint(), instanceName)
			if err != nil {
				return err
			}
		}

		if err := issueBuck2Token(ctx, tok, staticDur, &out); err != nil {
			return err
		}

		configuration := "remote execution"
		if !remote {
			configuration = "remote caching without remote execution"
		}

		return emitBuck2Config(ctx, out, buckConfigPath, output, configuration)
	})
}

type buck2Setup struct {
	EngineAddress      string        `json:"engine_address,omitempty"`
	ActionCacheAddress string        `json:"action_cache_address,omitempty"`
	CasAddress         string        `json:"cas_address,omitempty"`
	TLS                bool          `json:"tls"`
	InstanceName       string        `json:"instance_name,omitempty"`
	GrpcTimeout        time.Duration `json:"grpc_timeout,omitempty"`
	StaticToken        string        `json:"static_token,omitempty"`
	TokenDuration      time.Duration `json:"token_duration,omitempty"`
	RemoteExecution    bool          `json:"remote_execution"`
	Path               string        `json:"buckconfig_path,omitempty"`
}

func buck2TokenSource(ctx context.Context, tokenFile string) (api.TokenSource, error) {
	if tokenFile != "" {
		loaded, err := loadTokenFromFile(tokenFile)
		if err != nil {
			return nil, fnerrors.Newf("failed to load token from file: %w", err)
		}

		return loaded, nil
	}

	return fnapi.FetchToken(ctx)
}

func issueBuck2Token(ctx context.Context, tok api.TokenSource, dur time.Duration, out *buck2Setup) error {
	token, err := tok.IssueToken(ctx, dur, false)
	if err != nil {
		return fnerrors.Newf("failed to issue bearer token: %w", err)
	}

	if err := validateBuck2HeaderValue(token); err != nil {
		return err
	}

	out.StaticToken = token
	out.TokenDuration = dur

	return nil
}

func buck2CacheOnlySetup(endpoint, instanceName string) (buck2Setup, error) {
	address, tls, err := buck2Address(endpoint)
	if err != nil {
		return buck2Setup{}, err
	}

	// buck2 refuses to start its remote execution client unless engine_address
	// resolves, even when only the cache is used, so it points at the cache
	// endpoint too. Nothing is executed remotely as long as no execution
	// platform sets remote_enabled.
	return buck2Setup{
		EngineAddress:      address,
		ActionCacheAddress: address,
		CasAddress:         address,
		TLS:                tls,
		InstanceName:       instanceName,
	}, nil
}

func buck2ExecutionSetup(schedulerEndpoint, storageEndpoint, instanceName string) (buck2Setup, error) {
	engine, engineTLS, err := buck2Address(schedulerEndpoint)
	if err != nil {
		return buck2Setup{}, err
	}

	storage, storageTLS, err := buck2Address(storageEndpoint)
	if err != nil {
		return buck2Setup{}, err
	}

	if engineTLS != storageTLS {
		return buck2Setup{}, fnerrors.Newf("scheduler (%q) and storage (%q) endpoints disagree on TLS, which buck2 cannot express: it has a single tls setting for all endpoints", schedulerEndpoint, storageEndpoint)
	}

	return buck2Setup{
		EngineAddress:      engine,
		ActionCacheAddress: storage,
		CasAddress:         storage,
		TLS:                engineTLS,
		InstanceName:       instanceName,
		RemoteExecution:    true,
	}, nil
}

// buck2Address rewrites a Namespace endpoint into the address and TLS setting
// buck2 expects. buck2 rejects every URI scheme except grpc, dns, ipv4 and
// ipv6, and takes TLS from [buck2_re_client].tls rather than from the scheme.
func buck2Address(endpoint string) (string, bool, error) {
	scheme, host, found := strings.Cut(endpoint, "://")
	if !found {
		// Namespace endpoints are TLS-terminated, so assume TLS when the
		// server did not spell out a scheme.
		return "grpc://" + endpoint, true, nil
	}

	if host == "" {
		return "", false, fnerrors.Newf("endpoint %q has no host", endpoint)
	}

	switch scheme {
	case "grpcs", "https":
		return "grpc://" + host, true, nil
	case "grpc", "http":
		return "grpc://" + host, false, nil
	default:
		return "", false, fnerrors.Newf("cannot translate endpoint %q for buck2: unsupported scheme %q", endpoint, scheme)
	}
}

var buck2EnvVarRef = regexp.MustCompile(`\$[a-zA-Z_][a-zA-Z_0-9]*`)

// validateBuck2HeaderValue rejects tokens buck2 would not carry verbatim: it
// splits http_headers on commas, and expands $VAR references in each value
// against the environment.
func validateBuck2HeaderValue(token string) error {
	if strings.ContainsAny(token, ",\r\n") {
		return fnerrors.InternalError("token contains a comma or newline, which buck2 cannot carry in http_headers")
	}

	if match := buck2EnvVarRef.FindString(token); match != "" {
		return fnerrors.InternalError("token contains %q, which buck2 would expand as an environment variable", match)
	}

	return nil
}

func toBuck2Config(out buck2Setup) ([]byte, error) {
	if err := validateBuck2HeaderValue(out.StaticToken); err != nil {
		return nil, err
	}

	var buf bytes.Buffer

	fmt.Fprintf(&buf, "# Generated by nsc; do not edit.\n\n")
	fmt.Fprintf(&buf, "[buck2_re_client]\n")
	fmt.Fprintf(&buf, "engine_address = %s\n", out.EngineAddress)
	fmt.Fprintf(&buf, "action_cache_address = %s\n", out.ActionCacheAddress)
	fmt.Fprintf(&buf, "cas_address = %s\n", out.CasAddress)
	fmt.Fprintf(&buf, "tls = %t\n", out.TLS)

	if out.InstanceName != "" {
		fmt.Fprintf(&buf, "instance_name = %s\n", out.InstanceName)
	}

	if out.GrpcTimeout > 0 {
		fmt.Fprintf(&buf, "grpc_timeout = %d\n", int(out.GrpcTimeout.Seconds()))
	}

	if out.StaticToken != "" {
		fmt.Fprintf(&buf, "http_headers = %s:Bearer %s\n", buck2IngressAuthHeader, out.StaticToken)
	}

	return buf.Bytes(), nil
}

// resolveBuck2ConfigPath decides where to write the buckconfig. buck2 builds
// its remote execution configuration when the daemon starts, from its on-disk
// config files only, so there is no equivalent of bazel's --bazelrc: a file
// passed with --config-file never reaches [buck2_re_client]. The default is
// therefore a location buck2 already reads.
func resolveBuck2ConfigPath(buckConfigPath string) (string, bool, error) {
	if buckConfigPath != "" {
		return buckConfigPath, false, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", false, fnerrors.Newf("failed to determine the home directory: %w", err)
	}

	return filepath.Join(home, buck2ConfigDirName, buck2ConfigFileName), true, nil
}

// warnUnreadBuck2ConfigPath warns when an explicit destination is somewhere
// buck2 does not source configuration from.
func warnUnreadBuck2ConfigPath(ctx context.Context, path string) {
	switch filepath.Base(filepath.Dir(path)) {
	case buck2ConfigDirName, "buckconfig.d":
		return
	}

	switch filepath.Base(path) {
	case ".buckconfig", ".buckconfig.local", "buckconfig":
		return
	}

	fmt.Fprintf(console.Warnings(ctx), "buck2 does not read configuration from %s.\n", path)
	fmt.Fprintf(console.Warnings(ctx), "Remote execution can only be configured from .buckconfig, .buckconfig.local, a .buckconfig.d/ directory, or /etc/buckconfig.d/.\n")
}

func emitBuck2Config(ctx context.Context, out buck2Setup, buckConfigPath, output, configuration string) error {
	path, isDefault, err := resolveBuck2ConfigPath(buckConfigPath)
	if err != nil {
		return err
	}

	data, err := toBuck2Config(out)
	if err != nil {
		return err
	}

	if err := writeCredentialFile(path, data); err != nil {
		return err
	}

	out.Path = path

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

		if !isDefault {
			warnUnreadBuck2ConfigPath(ctx, path)
		}

		fmt.Fprintf(console.Stdout(ctx), "Wrote buckconfig for %s to %s.\n", configuration, path)

		if out.TokenDuration > 0 {
			fmt.Fprintf(console.Stdout(ctx), "Token valid for at least %s.\n", formatDuration(out.TokenDuration))
		}

		style := colors.Ctx(ctx)
		fmt.Fprintf(console.Stdout(ctx), "\nRestart the buck2 daemon to pick it up:\n")
		fmt.Fprintf(console.Stdout(ctx), "  %s\n", style.Highlight.Apply("buck2 killall"))
	}

	return nil
}

// writeCredentialFile writes a file that embeds a bearer token, so it is not
// readable by other users.
func writeCredentialFile(path string, content []byte) error {
	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fnerrors.Newf("failed to create directory %q: %w", dir, err)
		}
	}

	if err := os.WriteFile(path, content, 0600); err != nil {
		return fnerrors.Newf("failed to write %q: %w", path, err)
	}

	return nil
}
