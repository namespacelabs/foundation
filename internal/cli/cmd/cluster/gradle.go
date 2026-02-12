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
	"time"

	"buf.build/gen/go/namespace/cloud/connectrpc/go/proto/namespace/cloud/integrations/gradle/v1beta/gradlev1betaconnect"
	iamv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/iam/v1beta"
	gradlev1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/cloud/integrations/gradle/v1beta"
	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/framework/atomic"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const gradleCachePathBase = "gradle"

func NewGradleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gradle",
		Short: "Gradle-related activities.",
	}

	cache := &cobra.Command{Use: "cache", Short: "Gradle cache related functionality."}
	cache.AddCommand(newSetupGradleCacheCmd())
	cache.AddCommand(newGradleCreateTokenCmd())

	cmd.AddCommand(cache)

	return cmd
}

// NewGradleCacheCmd returns a "gradle" command with setup/create-token directly
// underneath, for use under "nsc cache gradle {setup|create-token}".
func NewGradleCacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gradle",
		Short: "Gradle cache related functionality.",
	}

	cmd.AddCommand(newSetupGradleCacheCmd())
	cmd.AddCommand(newGradleCreateTokenCmd())

	return cmd
}

func newSetupGradleCacheCmd() *cobra.Command {
	var initGradlePath, output string
	var push, user bool
	var name, site string
	var tokenFile string
	var flags *pflag.FlagSet

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote Gradle cache and generate an init.gradle to use it.",
		Long: `Set up a remote Gradle cache and generate an init.gradle to use it.

This command provisions a remote build cache and generates an init.gradle file
that configures Gradle to use it.

The generated init.gradle can be used with:
  gradle --init-script=/path/to/init.gradle build

Or by placing it in ~/.gradle/init.d/ to apply to all builds.`,
	}).WithFlags(func(f *pflag.FlagSet) {
		flags = f
		f.StringVar(&initGradlePath, "init-gradle", "", "If specified, write the init.gradle to this path.")
		f.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		f.BoolVar(&push, "push", true, "Whether to enable pushing to the cache (default: true).")
		f.StringVar(&name, "cache_name", "default", "A name for the cache.")
		f.StringVar(&name, "name", "default", "Deprecated: use --cache_name instead.")
		f.MarkHidden("name")
		f.StringVar(&site, "site", "", "Site preference (e.g., 'iad', 'fra'). If not set, determined automatically.")
		f.BoolVar(&user, "user", false, "If set, write the init.gradle to ~/.gradle/init.d/namespace.cache.gradle.")
		f.StringVar(&tokenFile, "token", "", "Use the bearer token stored at this location for authentication instead of the default.")
	}).Do(func(ctx context.Context) error {
		if flags.Changed("name") {
			fmt.Fprintf(console.Warnings(ctx), "--name is deprecated; use --cache_name\n")
		}

		if user && initGradlePath != "" {
			return fnerrors.New("--user and --init-gradle are mutually exclusive")
		}

		if user {
			gradleUserHome := os.Getenv("GRADLE_USER_HOME")
			if gradleUserHome == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fnerrors.Newf("failed to get user home directory: %w", err)
				}
				gradleUserHome = filepath.Join(homeDir, ".gradle")
			}
			initGradlePath = filepath.Join(gradleUserHome, "init.d", "namespace.cache.gradle")
		}

		var client gradlev1betaconnect.GradleCacheServiceClient
		if tokenFile != "" {
			tokenSource, err := loadTokenFromFile(tokenFile)
			if err != nil {
				return fnerrors.Newf("failed to load token from file: %w", err)
			}

			client = fnapi.NewGradleCacheServiceClientWithToken(tokenSource)
		} else {
			cli, err := fnapi.NewGradleCacheServiceClient(ctx)
			if err != nil {
				return err
			}

			client = cli
		}

		req := connect.NewRequest(&gradlev1beta.EnsureGradleCacheRequest{
			Name: name,
			Site: site,
		})

		resp, err := client.EnsureGradleCache(ctx, req)
		if err != nil {
			return fnerrors.Newf("failed to provision gradle cache: %w", err)
		}

		if resp.Msg.GetCacheEndpointUrl() == "" {
			return fnerrors.Newf("did not receive a valid cache endpoint")
		}

		var expiresAt *time.Time
		if resp.Msg.GetExpiresAt() != nil {
			t := resp.Msg.GetExpiresAt().AsTime()
			expiresAt = &t
		}

		out := gradleSetup{
			Endpoint:  resp.Msg.GetCacheEndpointUrl(),
			Username:  resp.Msg.GetUsername(),
			Password:  resp.Msg.GetPassword(),
			Push:      push,
			ExpiresAt: expiresAt,
			Site:      resp.Msg.GetSite(),
		}

		// Generate init.gradle file.
		if initGradlePath != "" {
			// Ensure the parent directory exists.
			if err := os.MkdirAll(filepath.Dir(initGradlePath), 0755); err != nil {
				return fnerrors.Newf("failed to create directory: %w", err)
			}

			data, err := toGradleInitScript(out)
			if err != nil {
				return err
			}

			if _, err := atomic.WriteFile(initGradlePath, bytes.NewReader(data)); err != nil {
				return fnerrors.Newf("failed to write init.gradle: %w", err)
			}
		}

		switch output {
		case "json":
			d := json.NewEncoder(console.Stdout(ctx))
			d.SetIndent("", "  ")
			if err := d.Encode(out); err != nil {
				return fnerrors.InternalError("failed to encode output as JSON: %w", err)
			}

		default:
			if output != "" && output != "plain" {
				fmt.Fprintf(console.Warnings(ctx), "unsupported output %q, defaulting to plain\n", output)
			}

			// For plain output, flush the state to a temp init.gradle if none is written yet.
			if initGradlePath == "" {
				data, err := toGradleInitScript(out)
				if err != nil {
					return err
				}

				loc, err := writeSecureTempFile(gradleCachePathBase, "*.init.gradle", data)
				if err != nil {
					return fnerrors.Newf("failed to create temp file: %w", err)
				}

				initGradlePath = loc
			}

			fmt.Fprintf(console.Stdout(ctx), "Wrote init.gradle configuration for remote cache to %s.\n", initGradlePath)

			if out.ExpiresAt != nil {
				remaining := time.Until(*out.ExpiresAt).Round(time.Minute)
				fmt.Fprintf(console.Stdout(ctx), "Configuration expires at %s (in %s).\n", out.ExpiresAt.Format(time.RFC3339), formatDuration(remaining))
			}

			if user {
				fmt.Fprintf(console.Stdout(ctx), "\nGradle will automatically use this configuration for all builds.\n")
			} else {
				style := colors.Ctx(ctx)
				fmt.Fprintf(console.Stdout(ctx), "\nStart using it by adding:\n")
				fmt.Fprintf(console.Stdout(ctx), "  %s\n", style.Highlight.Apply(fmt.Sprintf("--init-script=%s", initGradlePath)))
				fmt.Fprintf(console.Stdout(ctx), "\nOr copy it to ~/.gradle/init.d/ to apply to all builds.\n")
			}

			if tokenFile != "" {
				fmt.Fprintf(console.Stdout(ctx), "\nToken file %s can be removed now.\n", tokenFile)
			}
		}

		return nil
	})
}

func newGradleCreateTokenCmd() *cobra.Command {
	return newCreateCacheTokenCmd(createTokenConfig{
		Short:       "Create a revokable token for accessing the Gradle cache.",
		CacheLabel:  "Gradle",
		TokenPrefix: "gradle-cache",
		SetupCmd:    "nsc cache gradle setup",
		GetRequiredPerms: func(ctx context.Context, cacheName string) ([]*iamv1beta.Permission, error) {
			gradleClient, err := fnapi.NewGradleCacheServiceClient(ctx)
			if err != nil {
				return nil, err
			}

			policyResp, err := gradleClient.GetAccessPolicy(ctx, connect.NewRequest(&gradlev1beta.GetAccessPolicyRequest{
				Name: cacheName,
			}))
			if err != nil {
				return nil, fnerrors.Newf("failed to get access policy: %w", err)
			}

			return policyResp.Msg.GetRequiredPermission(), nil
		},
	})
}

func toGradleInitScript(out gradleSetup) ([]byte, error) {
	var buffer bytes.Buffer

	buffer.WriteString("// Generated by nsc gradle cache setup\n")
	buffer.WriteString("// This file configures Gradle to use Namespace's remote cache.\n")
	if out.ExpiresAt != nil {
		buffer.WriteString(fmt.Sprintf("// Expires at: %s\n", out.ExpiresAt.Format(time.RFC3339)))
	}
	buffer.WriteString("\n")

	buffer.WriteString("gradle.settingsEvaluated { settings ->\n")
	buffer.WriteString("    settings.buildCache {\n")
	buffer.WriteString("        remote(HttpBuildCache) {\n")
	buffer.WriteString(fmt.Sprintf("            url = '%s'\n", out.Endpoint))
	buffer.WriteString("            enabled = true\n")
	buffer.WriteString(fmt.Sprintf("            push = %t\n", out.Push))
	buffer.WriteString("            allowUntrustedServer = false\n")

	if out.Username != "" || out.Password != "" {
		buffer.WriteString("            credentials { creds ->\n")
		if out.Username != "" {
			buffer.WriteString(fmt.Sprintf("                creds.username = '%s'\n", out.Username))
		}
		if out.Password != "" {
			buffer.WriteString(fmt.Sprintf("                creds.password = '%s'\n", out.Password))
		}
		buffer.WriteString("            }\n")
	}

	buffer.WriteString("        }\n")
	buffer.WriteString("    }\n")
	buffer.WriteString("}\n")

	return buffer.Bytes(), nil
}

type gradleSetup struct {
	Endpoint  string     `json:"endpoint,omitempty"`
	Username  string     `json:"username,omitempty"`
	Password  string     `json:"password,omitempty"`
	Push      bool       `json:"push"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Site      string     `json:"site,omitempty"`
}

// writeSecureTempFile creates a temporary file in a secure temp directory that
// gets cleaned up on system reboot. This is suitable for files containing credentials.
func writeSecureTempFile(base, pattern string, content []byte) (string, error) {
	f, err := dirs.CreateSecureTemp(base, pattern)
	if err != nil {
		return "", fnerrors.Newf("failed to create secure temp file: %w", err)
	}

	if _, err := f.Write(content); err != nil {
		return "", fnerrors.Newf("failed to write to %s: %w", f.Name(), err)
	}

	if err := f.Close(); err != nil {
		return "", fnerrors.Newf("failed to close %s: %w", f.Name(), err)
	}

	return f.Name(), nil
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours >= 24 {
		days := hours / 24
		hours = hours % 24
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh%dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}

	return fmt.Sprintf("%dm", minutes)
}

type staticTokenSource string

func (s staticTokenSource) IssueToken(_ context.Context, _ time.Duration, _ bool) (string, error) {
	return string(s), nil
}

func loadTokenFromFile(path string) (staticTokenSource, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	var tj struct {
		BearerToken string `json:"bearer_token"`
	}

	if err := json.Unmarshal(contents, &tj); err != nil {
		return "", fnerrors.Newf("failed to parse token file: %w", err)
	}

	if tj.BearerToken == "" {
		return "", fnerrors.New("token file does not contain a bearer_token")
	}

	return staticTokenSource(tj.BearerToken), nil
}
