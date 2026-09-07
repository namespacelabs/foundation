// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	bazelv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/bazel/v1beta"
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
	reapiFlavorBazel = "bazel"
	reapiFlavorBuck2 = "buck2"
)

type reapiSetupOptions struct {
	key                  string
	storageMode          string
	static               bool
	staticTokenDuration  time.Duration
	enableRemoteAssetAPI bool
	execution            bool
}

type reapiSetupResult struct {
	bazelRbeSetup
	clientCertPEM []byte
	clientKeyPEM  []byte
}

func NewReapiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reapi",
		Short: "Remote Execution API related functionality.",
	}
	setup := &cobra.Command{
		Use:   "setup",
		Short: "Set up Remote Execution API storage or execution and write tool configuration.",
	}
	setup.AddCommand(newSetupReapiCmd(reapiFlavorBazel, true))
	setup.AddCommand(newSetupReapiCmd(reapiFlavorBuck2, true))
	cmd.AddCommand(setup)
	cmd.AddCommand(newReapiCreateTokenCmd())
	return cmd
}

func NewBuck2CacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buck2",
		Short: "Buck2 cache related functionality.",
	}
	cmd.AddCommand(newSetupReapiCmd(reapiFlavorBuck2, false))
	return cmd
}

func newSetupReapiCmd(flavor string, includeRemoteFlag bool) *cobra.Command {
	var configPath, output, bazelCommand, key, tokenFile, storageMode string
	var staticDuration time.Duration
	var static, enableRemoteAssetAPI, disableBuildEvents bool
	remote := includeRemoteFlag
	use := flavor
	if !includeRemoteFlag {
		use = "setup"
	}

	cmd := fncobra.Cmd(&cobra.Command{
		Use:   use,
		Short: fmt.Sprintf("Set up Remote Execution API storage or execution for %s.", flavor),
		Args:  cobra.NoArgs,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&configPath, "config", "", "Write the generated tool configuration to this path.")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.StringVar(&key, "key", "", "Stable identifier that disambiguates parallel execution or storage clusters. Defaults to 'default'.")
		flags.StringVar(&tokenFile, "token", "", "Use the bearer token stored at this location for authentication. Implies --static.")
		flags.BoolVar(&static, "static", false, "Authenticate with a static bearer token instead of an mTLS client certificate.")
		if includeRemoteFlag {
			flags.BoolVar(&remote, "remote", true, "Enable remote execution. If false, provision remote storage only.")
		}
		flags.StringVar(&storageMode, "storage", bazelStorageReadWrite, "Storage access mode. Valid options: read-only, read-write.")
		if flavor == reapiFlavorBazel {
			flags.StringVar(&bazelCommand, "command", defaultBazelRbeCommand, "Bazel command used in the generated bazelrc (e.g. build or common).")
			flags.BoolVar(&enableRemoteAssetAPI, "enable_remote_asset_api", false, "Enable the Remote Asset API.")
			flags.BoolVar(&disableBuildEvents, "disable_build_events", false, "Do not configure Bazel build event ingestion.")
		}
		fncobra.DurationVar(flags, &staticDuration, "static_token_duration", 4*time.Hour, "The minimum duration of the static token configured (requires --static).")
	}).Do(func(ctx context.Context) error {
		if err := validateReapiSetupFlags(output, storageMode, staticDuration, remote, enableRemoteAssetAPI); err != nil {
			return err
		}

		tok, err := reapiTokenSource(ctx, tokenFile)
		if err != nil {
			return err
		}
		if tokenFile != "" {
			static = true
		}

		result, err := setupReapi(ctx, tok, reapiSetupOptions{
			key:                  key,
			storageMode:          storageMode,
			static:               static,
			staticTokenDuration:  staticDuration,
			enableRemoteAssetAPI: enableRemoteAssetAPI,
			execution:            remote,
		})
		if err != nil {
			return err
		}

		switch flavor {
		case reapiFlavorBazel:
			return emitReapiBazelConfig(ctx, result, configPath, output, bazelCommand, remote, disableBuildEvents)
		case reapiFlavorBuck2:
			return emitReapiBuck2Config(ctx, result, configPath, output, remote, staticDuration)
		default:
			return fnerrors.InternalError("unhandled REAPI flavor %q", flavor)
		}
	})

	return cmd
}

func reapiTokenSource(ctx context.Context, tokenFile string) (api.TokenSource, error) {
	if tokenFile != "" {
		loaded, err := loadTokenFromFile(tokenFile)
		if err != nil {
			return nil, fnerrors.Newf("failed to load token from file: %w", err)
		}
		return loaded, nil
	}
	return fnapi.FetchToken(ctx)
}

func validateReapiSetupFlags(output, storageMode string, staticDuration time.Duration, remote, enableRemoteAssetAPI bool) error {
	if output != "plain" && output != "json" {
		return fnerrors.BadInputError("unsupported output format %q, supported values: plain, json", output)
	}
	if storageMode != bazelStorageReadOnly && storageMode != bazelStorageReadWrite {
		return fnerrors.BadInputError("invalid storage mode %q (valid values: read-only, read-write)", storageMode)
	}
	if remote && storageMode == bazelStorageReadOnly {
		return fnerrors.BadInputError("--storage=read-only requires --remote=false")
	}
	if enableRemoteAssetAPI && storageMode == bazelStorageReadOnly {
		return fnerrors.BadInputError("--enable_remote_asset_api may not be used with --storage=read-only")
	}
	if staticDuration <= 0 {
		return fnerrors.BadInputError("--static_token_duration must be greater than zero")
	}
	return nil
}

func setupReapi(ctx context.Context, tok api.TokenSource, opts reapiSetupOptions) (reapiSetupResult, error) {
	authMode := bazelv1beta.BazelExecutionAuthMode_BAZEL_EXECUTION_AUTH_MODE_MTLS
	if opts.static {
		authMode = bazelv1beta.BazelExecutionAuthMode_BAZEL_EXECUTION_AUTH_MODE_STATIC
	}

	var result reapiSetupResult
	if opts.execution {
		res, err := ensureBazelExecutionCluster(ctx, tok, opts.key, authMode, opts.enableRemoteAssetAPI)
		if err != nil {
			return reapiSetupResult{}, fnerrors.Newf("failed to provision REAPI execution cluster: %w", err)
		}
		if res.GetSchedulerEndpoint() == "" || res.GetStorageEndpoint() == "" {
			return reapiSetupResult{}, fnerrors.Newf("received incomplete response (scheduler=%q storage=%q)", res.GetSchedulerEndpoint(), res.GetStorageEndpoint())
		}

		result.bazelRbeSetup = bazelRbeSetup{
			SchedulerEndpoint:       res.GetSchedulerEndpoint(),
			StorageEndpoint:         res.GetStorageEndpoint(),
			RemoteAssetEndpoint:     res.GetRemoteAssetEndpoint(),
			Jobs:                    int32(res.GetRecommendedBazelJobs()),
			RemoteTimeout:           time.Duration(res.GetRecommendedBazelRemoteTimeoutSeconds()) * time.Second,
			RemoteLocalFallback:     res.GetRecommendedBazelRemoteLocalFallback(),
			RemoteDownloadOutputs:   res.GetRecommendedBazelRemoteDownloadOutputs(),
			BuildEventEndpoint:      res.GetBuildEventEndpoint(),
			BuildEventResultsURL:    res.GetBuildEventResultsUrl(),
			CredentialHelperDomains: res.GetCredentialHelperDomains(),
		}
	} else {
		res, err := ensureBazelStorageCluster(ctx, tok, opts.key, authMode, opts.enableRemoteAssetAPI, bazelStorageAccessMode(opts.storageMode))
		if err != nil {
			return reapiSetupResult{}, fnerrors.Newf("failed to provision REAPI storage cluster: %w", err)
		}
		if res.GetStorageEndpoint() == "" {
			return reapiSetupResult{}, fnerrors.Newf("received incomplete response (storage=%q)", res.GetStorageEndpoint())
		}
		result.bazelRbeSetup = bazelStorageSetup(res)
	}

	if opts.static {
		token, err := tok.IssueToken(ctx, opts.staticTokenDuration, false)
		if err != nil {
			return reapiSetupResult{}, fnerrors.Newf("failed to issue bearer token: %w", err)
		}
		if err := validateBuck2HeaderValue(token); err != nil {
			return reapiSetupResult{}, err
		}
		result.IngressAuthToken = token
	} else {
		privateKeyPEM, publicKeyPEM, err := genPrivAndPublicKeysPEM()
		if err != nil {
			return reapiSetupResult{}, fnerrors.Newf("failed to generate client key pair: %w", err)
		}
		privateKeyPEM, err = convertECPrivateKeyToPKCS8(privateKeyPEM)
		if err != nil {
			return reapiSetupResult{}, fnerrors.Newf("failed to encode client key in PKCS#8: %w", err)
		}
		clientCertPEM, err := fetchTenantClientCert(ctx, string(publicKeyPEM))
		if err != nil {
			return reapiSetupResult{}, fnerrors.Newf("failed to issue client certificate: %w", err)
		}
		result.clientCertPEM = []byte(clientCertPEM)
		result.clientKeyPEM = privateKeyPEM
	}

	readiness, cleanup, err := reapiReadinessConfig(result)
	if err != nil {
		return reapiSetupResult{}, err
	}
	defer cleanup()
	if err := waitForBazelCacheReady(ctx, readiness); err != nil {
		return reapiSetupResult{}, fnerrors.Newf("failed waiting for REAPI storage readiness: %w", err)
	}

	return result, nil
}

func reapiReadinessConfig(result reapiSetupResult) (bazelCacheReadinessConfig, func(), error) {
	cfg := bazelCacheReadinessConfig{
		endpoint:    result.StorageEndpoint,
		bearerToken: result.IngressAuthToken,
		waitTimeout: bazelCacheReadinessTimeout,
	}
	if len(result.clientCertPEM) == 0 {
		if len(result.clientKeyPEM) != 0 {
			return bazelCacheReadinessConfig{}, nil, fnerrors.New("received a client key without a client certificate")
		}
		return cfg, func() {}, nil
	}
	if len(result.clientKeyPEM) == 0 {
		return bazelCacheReadinessConfig{}, nil, fnerrors.New("received a client certificate without a client key")
	}

	cert, err := os.CreateTemp("", "nsc-reapi-client-*.cert")
	if err != nil {
		return bazelCacheReadinessConfig{}, nil, fnerrors.Newf("failed to create temporary client certificate: %w", err)
	}
	key, err := os.CreateTemp("", "nsc-reapi-client-*.key")
	if err != nil {
		_ = os.Remove(cert.Name())
		return bazelCacheReadinessConfig{}, nil, fnerrors.Newf("failed to create temporary client key: %w", err)
	}
	cleanup := func() {
		_ = os.Remove(cert.Name())
		_ = os.Remove(key.Name())
	}
	if _, err := cert.Write(result.clientCertPEM); err != nil {
		_ = cert.Close()
		_ = key.Close()
		cleanup()
		return bazelCacheReadinessConfig{}, nil, fnerrors.Newf("failed to write temporary client certificate: %w", err)
	}
	if _, err := key.Write(result.clientKeyPEM); err != nil {
		_ = cert.Close()
		_ = key.Close()
		cleanup()
		return bazelCacheReadinessConfig{}, nil, fnerrors.Newf("failed to write temporary client key: %w", err)
	}
	if err := cert.Close(); err != nil {
		_ = key.Close()
		cleanup()
		return bazelCacheReadinessConfig{}, nil, fnerrors.Newf("failed to close temporary client certificate: %w", err)
	}
	if err := key.Close(); err != nil {
		cleanup()
		return bazelCacheReadinessConfig{}, nil, fnerrors.Newf("failed to close temporary client key: %w", err)
	}

	cfg.clientCert = cert.Name()
	cfg.clientKey = key.Name()
	return cfg, cleanup, nil
}

func emitReapiBazelConfig(ctx context.Context, result reapiSetupResult, configPath, output, command string, execution, disableBuildEvents bool) error {
	if len(result.clientCertPEM) > 0 {
		path, err := writeTempFile(bazelExecutionPathBase, "*.cert", result.clientCertPEM)
		if err != nil {
			return fnerrors.Newf("failed to write client certificate: %w", err)
		}
		result.ClientCert = path
		path, err = writeTempFile(bazelExecutionPathBase, "*.key", result.clientKeyPEM)
		if err != nil {
			return fnerrors.Newf("failed to write client key: %w", err)
		}
		result.ClientKey = path
	}

	data, err := toBazelExecutionConfig(ctx, result.bazelRbeSetup, command, execution, disableBuildEvents)
	if err != nil {
		return err
	}
	if configPath == "" {
		configPath, err = writeTempFile(bazelExecutionPathBase, "*.bazelrc", data)
	} else {
		err = writeCredentialFile(configPath, data)
	}
	if err != nil {
		return fnerrors.Newf("failed to write bazelrc: %w", err)
	}

	if output == "json" {
		return encodeReapiOutput(ctx, reapiFlavorBazel, configPath, execution, result.bazelRbeSetup)
	}
	configuration := "remote execution"
	if !execution {
		configuration = "remote caching without remote execution"
	}
	fmt.Fprintf(console.Stdout(ctx), "Wrote Bazel configuration for %s to %s.\n", configuration, configPath)
	style := colors.Ctx(ctx)
	fmt.Fprintf(console.Stdout(ctx), "\nStart using it by adding:\n  %s", style.Highlight.Apply(fmt.Sprintf("--bazelrc=%s\n", configPath)))
	return nil
}

func emitReapiBuck2Config(ctx context.Context, result reapiSetupResult, configPath, output string, execution bool, staticDuration time.Duration) error {
	var out buck2Setup
	var err error
	if execution {
		out, err = buck2ExecutionSetup(result.SchedulerEndpoint, result.StorageEndpoint, "")
	} else {
		out, err = buck2CacheOnlySetup(result.StorageEndpoint, "")
	}
	if err != nil {
		return err
	}
	if result.IngressAuthToken != "" {
		out.StaticToken = result.IngressAuthToken
		out.TokenDuration = staticDuration
	} else if err := setBuck2ClientIdentity(&out, result.clientCertPEM, result.clientKeyPEM); err != nil {
		return err
	}

	configuration := "remote execution"
	if !execution {
		configuration = "remote caching without remote execution"
	}
	return emitBuck2Config(ctx, out, configPath, output, configuration)
}

func encodeReapiOutput(ctx context.Context, flavor, configPath string, execution bool, setup any) error {
	out := struct {
		Flavor          string `json:"flavor"`
		ConfigPath      string `json:"config_path"`
		RemoteExecution bool   `json:"remote_execution"`
		Setup           any    `json:"setup"`
	}{flavor, configPath, execution, setup}
	encoder := json.NewEncoder(console.Stdout(ctx))
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(out); err != nil {
		return fnerrors.InternalError("failed to encode response as JSON: %w", err)
	}
	return nil
}
