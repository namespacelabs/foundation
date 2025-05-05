// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package aws

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newSetupWebIdentity() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "setup-web-identity",
		Short:  "Provides configuration to the AWS CLI/SDK to use Workload Federation to access a particular AWS role.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	roleArn := cmd.Flags().String("role_arn", "", "The ARN of the role to assume.")
	envFile := cmd.Flags().String("write_env", "", "Instead of outputting, write the environment variables to the specified file.")
	tokenDir := cmd.Flags().String("token_dir", "", "If specified, stores any obtained tokens here.")
	duration := cmd.Flags().Duration("duration", 0, "How long the token should last.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *roleArn == "" {
			return fnerrors.Newf("--role_arn is required")
		}

		token, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		claims, err := token.Claims(ctx)
		if err != nil {
			return err
		}

		var out = console.Stdout(ctx)
		if *envFile != "" {
			f, err := os.Create(*envFile)
			if err != nil {
				return err
			}
			defer f.Close()

			out = f
		}

		tokenFile, err := os.CreateTemp(*tokenDir, "idtoken")
		if err != nil {
			return err
		}

		defer tokenFile.Close()

		t := time.Now()
		resp, err := fnapi.IssueIdToken(ctx, "sts.amazonaws.com", idTokenVersion, *duration)
		if err != nil {
			return err
		}

		if _, err := tokenFile.Write([]byte(resp.IdToken)); err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Obtained credentials from Namespace (took %v).\n", time.Since(t))

		sessionName := ""
		if claims.InstanceID != "" {
			sessionName = fmt.Sprintf("nsc.instance=%s", claims.InstanceID)
		} else {
			sessionName = fmt.Sprintf("nsc.tenant=%s", claims.TenantID)
		}

		fmt.Fprintf(out, "export AWS_ROLE_ARN=%s\n", *roleArn)
		fmt.Fprintf(out, "export AWS_ROLE_SESSION_NAME=%s\n", sessionName)
		fmt.Fprintf(out, "export AWS_WEB_IDENTITY_TOKEN_FILE=%s\n", tokenFile.Name())

		if *envFile != "" {
			fmt.Fprintf(console.Stderr(ctx), "Wrote %s.\n", *envFile)
		}

		return nil
	})
}
