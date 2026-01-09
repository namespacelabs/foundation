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
	"google.golang.org/protobuf/types/known/timestamppb"
	"namespacelabs.dev/foundation/framework/atomic"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/colors"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/go-ids"
	"namespacelabs.dev/integrations/api/iam"
	"namespacelabs.dev/integrations/auth"
)

const gradleCachePathBase = "gradle"

func NewGradleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gradle",
		Short: "Gradle-related activities.",
	}

	cache := &cobra.Command{Use: "cache", Short: "Gradle cache related functionality."}
	cache.AddCommand(newSetupGradleCacheCmd())
	cache.AddCommand(newCreateTokenCmd())

	cmd.AddCommand(cache)

	return cmd
}

func newSetupGradleCacheCmd() *cobra.Command {
	var initGradlePath, output string
	var push, user bool
	var name, site string
	var tokenFile string

	return fncobra.Cmd(&cobra.Command{
		Use:   "setup",
		Short: "Set up a remote Gradle cache and generate an init.gradle to use it.",
		Long: `Set up a remote Gradle cache and generate an init.gradle to use it.

This command provisions a remote build cache and generates an init.gradle file
that configures Gradle to use it.

The generated init.gradle can be used with:
  gradle --init-script=/path/to/init.gradle build

Or by placing it in ~/.gradle/init.d/ to apply to all builds.`,
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&initGradlePath, "init-gradle", "", "If specified, write the init.gradle to this path.")
		flags.StringVarP(&output, "output", "o", "plain", "One of plain or json.")
		flags.BoolVar(&push, "push", true, "Whether to enable pushing to the cache (default: true).")
		flags.StringVar(&name, "name", "default", "A name for the cache.")
		flags.StringVar(&site, "site", "", "Site preference (e.g., 'iad', 'fra'). If not set, determined automatically.")
		flags.BoolVar(&user, "user", false, "If set, write the init.gradle to ~/.gradle/init.d/namespace.cache.gradle.")
		flags.StringVar(&tokenFile, "token", "", "Use the bearer token stored at this location for authentication instead of the default.")
	}).Do(func(ctx context.Context) error {
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

func newCreateTokenCmd() *cobra.Command {
	var name string
	var expiresIn time.Duration
	var tokenFile, scope string

	return fncobra.Cmd(&cobra.Command{
		Use:   "create-token",
		Short: "Create a revokable token for accessing the Gradle cache.",
	}).WithFlags(func(flags *pflag.FlagSet) {
		flags.StringVar(&name, "cache_name", "", "Select a Gradle cache to grant access to. By default, all Gradle caches can be accessed.")
		fncobra.DurationVar(flags, &expiresIn, "expires_in", 24*time.Hour, "Duration until the token expires (max 90 days).")
		flags.StringVar(&tokenFile, "token", "token.json", "Write token to this file in JSON format.")
		flags.StringVar(&scope, "scope", "user", "Set the scope of the generated access token. Valid options: `tenant`, `user`. Tokens with user scope are bound to the tenant membership of the current user.")
	}).Do(func(ctx context.Context) error {
		gradleClient, err := fnapi.NewGradleCacheServiceClient(ctx)
		if err != nil {
			return err
		}

		policyResp, err := gradleClient.GetAccessPolicy(ctx, connect.NewRequest(&gradlev1beta.GetAccessPolicyRequest{
			Name: name,
		}))
		if err != nil {
			return fnerrors.Newf("failed to get access policy: %w", err)
		}

		requiredPerms := policyResp.Msg.GetRequiredPermission()
		if len(requiredPerms) == 0 {
			return fnerrors.New("no permissions required for this cache (unexpected)")
		}

		tokenSource, err := auth.LoadDefaults()
		if err != nil {
			return fnerrors.InvocationError("gradle", "failed to get authentication token: %w", err)
		}

		iamClient, err := iam.NewClient(ctx, tokenSource)
		if err != nil {
			return fnerrors.InvocationError("gradle", "failed to create IAM client: %w", err)
		}
		defer iamClient.Close()

		suffix := ids.NewRandomBase32ID(4)
		tokenName := fmt.Sprintf("gradle-cache-%s-%s", name, suffix)
		expiresAt := time.Now().Add(expiresIn)

		req := &iamv1beta.CreateRevokableTokenRequest{
			Name:        tokenName,
			Description: fmt.Sprintf("Gradle cache access token for cache %q", name),
			ExpiresAt:   timestamppb.New(expiresAt),
			Access: &iamv1beta.AccessPolicy{
				Grants: requiredPerms,
			},
		}

		switch scope {
		case "tenant":
			req.Scope = iamv1beta.RevokableToken_TENANT_SCOPE

		case "user":
			req.Scope = iamv1beta.RevokableToken_TENANT_MEMBERSHIP_SCOPE
		}

		resp, err := iamClient.Tokens.CreateRevokableToken(ctx, req)
		if err != nil {
			return fnerrors.InvocationError("token", "failed to create token: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "Token ID:    %s\n", resp.Token.GetTokenId())
		fmt.Fprintf(console.Stdout(ctx), "Name:        %s\n", resp.Token.GetName())
		fmt.Fprintf(console.Stdout(ctx), "Expires At:  %s\n", expiresAt.Format(time.RFC3339))

		if err := writeGradleTokenToFile(tokenFile, resp.BearerToken); err != nil {
			return fnerrors.InvocationError("token", "failed to write token to file: %w", err)
		}

		fmt.Fprintf(console.Stdout(ctx), "You can set up your gradle cache config with:\n")

		style := colors.Ctx(ctx)
		cmd := fmt.Sprintf("nsc gradle cache setup --token %s", tokenFile)
		if name != "" {
			cmd = fmt.Sprintf("%s --name %s", cmd, name)
		}

		fmt.Fprintf(console.Stdout(ctx), "  %s\n", style.Highlight.Apply(cmd))

		return nil
	})
}

func writeGradleTokenToFile(path string, bearerToken string) error {
	tokenData := map[string]string{
		"bearer_token": bearerToken,
	}

	bb, err := json.MarshalIndent(tokenData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, bb, 0600)
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
