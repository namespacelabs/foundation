// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const bazelCachePathBase = "bazelcache"

func NewBazelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bazel",
		Short: "Bazel-related activities.",
	}

	cache := &cobra.Command{Use: "cache", Short: "Bazel cache related functionality."}
	cache.AddCommand(newSetupCacheCmd())

	cmd.AddCommand(cache)

	return cmd
}

// NewBazelCacheCmd returns a "bazel" command with setup directly
// underneath, for use under "nsc cache bazel setup".
func NewBazelCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bazel",
		Short: "Bazel cache related functionality.",
	}

	cmd.AddCommand(newSetupCacheCmd())

	return cmd
}

const defaultBazelCommand = "build"

func newSetupCacheCmd() *cobra.Command {
	var bazelRcPath, output, certPath, bazelCommand string
	var experimentalCacheName string
	var sendBuildEvents, useAbsoluteCredHelperPath, static, experimentalDirect, enableRemoteAssetAPI bool
	var version int64
	var staticDur time.Duration

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote Bazel cache and generate a bazelrc to use it.",
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&bazelRcPath, "bazelrc", "", "If specified, write the bazelrc to this path.")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.StringVar(&certPath, "cred_path", "", "If specified, write credentials to this directory. Using this flag also ensures stable file names for all emitted credentials.")
		flags.BoolVar(&sendBuildEvents, "send_build_events", false, "If specified, send build events to the build event service.")
		flags.BoolVar(&useAbsoluteCredHelperPath, "use_absolute_credentialhelper_path", false, "If specified, use an absolute path to the credential helper binary.")
		flags.BoolVar(&static, "static", false, "If specified, use a static bearer token in --remote_header instead of a credential helper.")
		flags.BoolVar(&experimentalDirect, "experimental_direct", false, "If specified, configure Bazel to connect directly to the cache endpoint with a freshly issued client certificate.")
		flags.StringVar(&experimentalCacheName, "experimental_cache_name", "", "If specified, request a named experimental Bazel cache backing instance.")
		flags.Int64Var(&version, "version", 1, "Which bazel version to use.")
		fncobra.DurationVar(flags, &staticDur, "static_token_duration", 4*time.Hour, "The minimum duration of the static token configured (requires --static).")
		flags.StringVar(&bazelCommand, "command", defaultBazelCommand, "The bazel command to use in the generated bazelrc (e.g., 'build' or 'common').")
		flags.BoolVar(&enableRemoteAssetAPI, "enable_remote_asset_api", false, "If specified, opt-in to the remote asset API.")

		flags.MarkHidden("cred_path")
		flags.MarkHidden("use_absolute_credentialhelper_path")
		flags.MarkHidden("send_build_events")
		flags.MarkHidden("version")
	}).Do(func(ctx context.Context) error {
		client, err := fnapi.NewBazelCacheServiceClient(ctx)
		if err != nil {
			return err
		}

		if experimentalDirect && static {
			return fnerrors.Newf("--experimental_direct may not be used with --static")
		}

		if experimentalDirect && sendBuildEvents {
			return fnerrors.Newf("--experimental_direct may not be used with --send_build_events")
		}

		msg := makeEnsureBazelCacheRequest(version, experimentalDirect, enableRemoteAssetAPI, experimentalCacheName)

		req := connect.NewRequest(msg)

		resp, err := client.EnsureBazelCache(ctx, req)
		if err != nil {
			return fnerrors.Newf("failed to provision bazel cache: %w", err)
		}

		response := resp.Msg
		if response.GetCacheEndpoint() == "" {
			return fnerrors.Newf("did not receive a valid cache endpoint")
		}

		useWorkloadMtls := response.GetUseWorkloadMtls()
		if useWorkloadMtls && static {
			return fnerrors.Newf("server requires workload mTLS; --static may not be used")
		}

		if useWorkloadMtls && sendBuildEvents {
			return fnerrors.Newf("server requires workload mTLS; --send_build_events may not be used")
		}

		if certPath != "" {
			if err := os.MkdirAll(certPath, 0755); err != nil {
				return fnerrors.Newf("failed to create certificate directory: %w", err)
			}
		}

		var expiresAt *time.Time
		if response.GetExpiresAt() != nil {
			t := response.GetExpiresAt().AsTime()
			expiresAt = &t
		}

		out := baseBazelSetup(response, expiresAt)

		// The three auth modes below are mutually exclusive: workload mTLS
		// (client cert issued on the fly), credential helper (auth handled by
		// an external binary per request), or legacy TLS material returned in
		// the response. Pick exactly one.
		switch {
		case useWorkloadMtls:
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

			if certPath == "" {
				loc, err := writeTempFile(bazelCachePathBase, "*.cert", []byte(clientCertPem))
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				out.ClientCert = loc
			} else {
				out.ClientCert = filepath.Join(certPath, "client.cert")

				if err := writeFile(out.ClientCert, []byte(clientCertPem)); err != nil {
					return err
				}
			}

			if certPath == "" {
				loc, err := writeTempFile(bazelCachePathBase, "*.key", privateKeyPem)
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				out.ClientKey = loc
			} else {
				out.ClientKey = filepath.Join(certPath, "client.key")

				if err := writeFile(out.ClientKey, privateKeyPem); err != nil {
					return err
				}
			}

		case len(response.GetCredentialHelperDomains()) > 0:
			// Credential helper is the sole auth mechanism; baseBazelSetup
			// already populated CredentialHelperDomains and the HTTPS endpoint.
			// Do not also emit TLS client cert/key/CA.

		default:
			if len(response.GetServerCaPem()) > 0 {
				if certPath == "" {
					loc, err := writeTempFile(bazelCachePathBase, "*.cert", []byte(response.GetServerCaPem()))
					if err != nil {
						return fnerrors.Newf("failed to create temp file: %w", err)
					}

					out.ServerCaCert = loc
				} else {
					out.ServerCaCert = filepath.Join(certPath, "server_ca.cert")

					if err := writeFile(out.ServerCaCert, []byte(response.GetServerCaPem())); err != nil {
						return err
					}
				}
			}

			if len(response.GetClientCertPem()) > 0 {
				if certPath == "" {
					loc, err := writeTempFile(bazelCachePathBase, "*.cert", []byte(response.GetClientCertPem()))
					if err != nil {
						return fnerrors.Newf("failed to create temp file: %w", err)
					}

					out.ClientCert = loc
				} else {
					out.ClientCert = filepath.Join(certPath, "client.cert")

					if err := writeFile(out.ClientCert, []byte(response.GetClientCertPem())); err != nil {
						return err
					}
				}
			}

			if len(response.GetClientKeyPem()) > 0 {
				if certPath == "" {
					loc, err := writeTempFile(bazelCachePathBase, "*.key", []byte(response.GetClientKeyPem()))
					if err != nil {
						return fnerrors.Newf("failed to create temp file: %w", err)
					}

					out.ClientKey = loc
				} else {
					out.ClientKey = filepath.Join(certPath, "client.key")

					if err := writeFile(out.ClientKey, []byte(response.GetClientKeyPem())); err != nil {
						return err
					}
				}
			}
		}

		if static {
			if response.GetHttpsCacheEndpoint() == "" {
				return fnerrors.Newf("--static requires HTTPS cache endpoint but it was not provided")
			}

			token, err := fnapi.IssueToken(ctx, staticDur)
			if err != nil {
				return fnerrors.Newf("failed to issue bearer token: %w", err)
			}

			out = bazelSetup{
				Endpoint:    response.GetHttpsCacheEndpoint(),
				StaticToken: token,
			}
		}

		if sendBuildEvents {
			if response.GetBuildEventEndpoint() == "" {
				return fnerrors.Newf("did not receive a valid build events endpoint but was asked to send build events")
			}

			if len(response.GetCredentialHelperDomains()) == 0 {
				return fnerrors.Newf("the credential helper is not enabled but it is required to send build events")
			}

			out.BuildEventEndpoint = response.GetBuildEventEndpoint()
		}

		if response.GetRemoteAssetEndpoint() != "" {
			out.RemoteAssetEndpoint = response.GetRemoteAssetEndpoint()
		}

		// If set, we always generate a bazelrc file.
		if bazelRcPath != "" {
			data, err := toBazelConfig(ctx, out, useAbsoluteCredHelperPath, bazelCommand)
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
				return fnerrors.InternalError("failed to encode token as JSON output: %w", err)
			}

		default:
			if output != "" && output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
			}

			// For plain output, flush the state to a temp bazelrc if none is written yet.
			if bazelRcPath == "" {
				data, err := toBazelConfig(ctx, out, useAbsoluteCredHelperPath, bazelCommand)
				if err != nil {
					return err
				}

				loc, err := writeTempFile(bazelCachePathBase, "*.bazelrc", data)
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				bazelRcPath = loc
			}

			fmt.Fprintf(console.Stdout(ctx), "Wrote bazelrc configuration for remote cache to %s.\n", bazelRcPath)

			style := colors.Ctx(ctx)
			fmt.Fprintf(console.Stdout(ctx), "\nStart using it by adding:\n")
			fmt.Fprintf(console.Stdout(ctx), "  %s", style.Highlight.Apply(fmt.Sprintf("--bazelrc=%s\n", bazelRcPath)))
		}

		return nil
	})
}

func makeEnsureBazelCacheRequest(version int64, experimentalDirect, enableRemoteAsset bool, experimentalCacheName string) *bazelv1beta.EnsureBazelCacheRequest {
	msg := &bazelv1beta.EnsureBazelCacheRequest{}
	msg.SetVersion(version)
	msg.SetExperimentalDirectMtls(experimentalDirect)
	msg.SetExperimentalCacheName(experimentalCacheName)
	msg.SetEnableRemoteAssetApi(enableRemoteAsset)

	return msg
}

func baseBazelSetup(response *bazelv1beta.EnsureBazelCacheResponse, expiresAt *time.Time) bazelSetup {
	out := bazelSetup{
		Endpoint:  response.GetCacheEndpoint(),
		ExpiresAt: expiresAt,
	}

	if len(response.GetCredentialHelperDomains()) > 0 && !response.GetUseWorkloadMtls() {
		out = bazelSetup{
			Endpoint:                response.GetHttpsCacheEndpoint(),
			ExpiresAt:               expiresAt,
			CredentialHelperDomains: response.GetCredentialHelperDomains(),
		}
	}

	return out
}

func convertECPrivateKeyToPKCS8(privateKeyPem []byte) ([]byte, error) {
	block, _ := pem.Decode(privateKeyPem)
	if block == nil {
		return nil, fnerrors.New("failed to decode private key PEM")
	}

	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	privateKeyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, err
	}

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyBytes}), nil
}

func writeTempFile(base, pattern string, content []byte) (string, error) {
	f, err := dirs.CreateUserTemp(base, pattern)
	if err != nil {
		return "", fnerrors.Newf("failed to create temp file: %w", err)
	}

	if _, err := f.Write(content); err != nil {
		return "", fnerrors.Newf("failed to write to %s: %w", f.Name(), err)
	}

	if err := f.Close(); err != nil {
		return "", fnerrors.Newf("failed to close %s: %w", f.Name(), err)
	}

	return f.Name(), nil
}

func writeFile(path string, content []byte) error {
	if err := os.WriteFile(path, content, 0644); err != nil {
		return fnerrors.Newf("failed to write %q: %w", path, err)
	}

	return nil
}

func toBazelConfig(ctx context.Context, out bazelSetup, useAbsoluteCredHelperPath bool, command string) ([]byte, error) {
	var buffer bytes.Buffer
	if _, err := buffer.WriteString(fmt.Sprintf("%s --remote_cache=%s\n", command, out.Endpoint)); err != nil {
		return nil, fnerrors.Newf("failed to append cache endpoint: %w", err)
	}

	if len(out.ServerCaCert) > 0 {
		if _, err := buffer.WriteString(fmt.Sprintf("%s --tls_certificate=%s\n", command, out.ServerCaCert)); err != nil {
			return nil, fnerrors.Newf("failed to append tls_certificate config: %w", err)
		}
	}

	if len(out.ClientCert) > 0 {
		if _, err := buffer.WriteString(fmt.Sprintf("%s --tls_client_certificate=%s\n", command, out.ClientCert)); err != nil {
			return nil, fnerrors.Newf("failed to append tls_client_certificate config: %w", err)
		}
	}

	if len(out.ClientKey) > 0 {
		if _, err := buffer.WriteString(fmt.Sprintf("%s --tls_client_key=%s\n", command, out.ClientKey)); err != nil {
			return nil, fnerrors.Newf("failed to append tls_client_key config: %w", err)
		}
	}

	if out.BuildEventEndpoint != "" {
		if _, err := buffer.WriteString(fmt.Sprintf("%s --bes_backend=%s\n", command, out.BuildEventEndpoint)); err != nil {
			return nil, fnerrors.Newf("failed to append bes_backend: %w", err)
		}
	}

	if out.RemoteAssetEndpoint != "" {
		if _, err := buffer.WriteString(fmt.Sprintf("%s --experimental_remote_downloader=%s\n", command, out.RemoteAssetEndpoint)); err != nil {
			return nil, fnerrors.Newf("failed to append experimental_remote_downloader: %w", err)
		}
	}

	if out.StaticToken != "" {
		if _, err := buffer.WriteString(fmt.Sprintf("%s --remote_header=x-nsc-ingress-auth=Bearer\\ %s\n", command, out.StaticToken)); err != nil {
			return nil, fnerrors.Newf("failed to append x-nsc-ingress-auth header: %w", err)
		}
	} else if len(out.CredentialHelperDomains) > 0 {
		path, err := exec.LookPath(BazelCredHelperBinary)
		if err != nil {
			stdout := console.Stdout(ctx)
			style := colors.Ctx(ctx)

			if errors.Is(err, exec.ErrNotFound) {
				fmt.Fprintln(stdout)
				fmt.Fprint(stdout, style.Highlight.Apply(fmt.Sprintf("We didn't find %s in your $PATH.", BazelCredHelperBinary)))
				fmt.Fprintf(stdout, "\nIt's usually installed along-side nsc; so if you have added nsc to the $PATH, %s will also be available.\n", BazelCredHelperBinary)
				fmt.Fprintf(stdout, "\nWhile your $PATH is not updated, accessing the remote bazel cache won't work.\n")
			}
			if !errors.Is(err, exec.ErrNotFound) || useAbsoluteCredHelperPath {
				return nil, fnerrors.Newf("failed to look up %s in $PATH: %w", BazelCredHelperBinary, err)
			}
		}

		for _, domain := range out.CredentialHelperDomains {
			credHelper := BazelCredHelperBinary
			if useAbsoluteCredHelperPath {
				credHelper = path
			}
			if _, err := buffer.WriteString(fmt.Sprintf("%s --credential_helper=*.%s=%s\n", command, domain, credHelper)); err != nil {
				return nil, fnerrors.Newf("failed to append credential_helper: %w", err)
			}
		}
	}

	return buffer.Bytes(), nil
}

type bazelSetup struct {
	Endpoint                string     `json:"endpoint,omitempty"`
	ServerCaCert            string     `json:"server_ca_cert,omitempty"`
	ClientCert              string     `json:"client_cert,omitempty"`
	ClientKey               string     `json:"client_key,omitempty"`
	ExpiresAt               *time.Time `json:"expires_at,omitempty"`
	CredentialHelperDomains []string   `json:"credential_helper_domains,omitempty"`
	BuildEventEndpoint      string     `json:"build_event_endpoint,omitempty"`
	RemoteAssetEndpoint     string     `json:"remote_asset_endpoint,omitempty"`
	StaticToken             string     `json:"static_token,omitempty"`
}
