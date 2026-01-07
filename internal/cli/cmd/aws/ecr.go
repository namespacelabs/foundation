// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package aws

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	dockercfg "github.com/docker/cli/cli/config"
	"github.com/docker/cli/cli/config/types"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const dockerCfgName = "config.json"

func newEcrDockerLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "ecr-docker-login",
		Short:  "Uses Workload Federation to log into ECR for use with Docker.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	roleArn := cmd.Flags().String("role_arn", "", "The ARN of the role to log in as.")
	awsProfile := cmd.Flags().String("aws_profile", "", "Use the specified AWS profile.")
	region := cmd.Flags().String("region", "", "Use the specified AWS region.")
	duration := fncobra.Duration(cmd.Flags(), "duration", time.Hour, "For how long the resulting AWS credentials should be valid for.")
	temp := cmd.Flags().Bool("temp", false, "Create a temporary Docker config file with the added credentials.")
	outputPath := cmd.Flags().String("output_to", "", "If specified, write the path of the Docker config to this path.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *roleArn == "" {
			return fnerrors.Newf("--role_arn is required")
		}

		arn, err := arn.Parse(*roleArn)
		if err != nil {
			return fnerrors.Newf("--role_arn is invalid: %w", err)
		}

		token, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		claims, err := token.Claims(ctx)
		if err != nil {
			return err
		}

		t := time.Now()
		resp, err := fnapi.IssueIdToken(ctx, "sts.amazonaws.com", idTokenVersion, *duration)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Obtained credentials from Namespace (took %v).\n", time.Since(t))

		client, err := newSTSClient(ctx, *awsProfile)
		if err != nil {
			return err
		}

		sessionName := ""
		if claims.InstanceID != "" {
			sessionName = fmt.Sprintf("nsc.instance=%s", claims.InstanceID)
		} else {
			sessionName = fmt.Sprintf("nsc.tenant=%s", claims.TenantID)
		}

		t = time.Now()
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(*region),
			config.WithCredentialsProvider(
				stscreds.NewWebIdentityRoleProvider(client, *roleArn, staticToken{resp.IdToken}, func(o *stscreds.WebIdentityRoleOptions) {
					o.RoleSessionName = sessionName
					o.Duration = *duration
				}),
			))
		if err != nil {
			return err
		}

		ecrToken, err := ecr.NewFromConfig(cfg).GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
		if err != nil {
			return err
		}

		if len(ecrToken.AuthorizationData) == 0 {
			return fnerrors.Newf("invalid response from ECR: missing authorization data")
		}

		authData := ecrToken.AuthorizationData[0].AuthorizationToken
		data, err := base64.StdEncoding.DecodeString(*authData)
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Obtained credentials from AWS (took=%v).\n", time.Since(t))

		parts := strings.SplitN(string(data), ":", 2)
		if len(parts) != 2 {
			return fnerrors.Newf("invalid response from ECR: authorization token has %d parts", len(parts))
		}

		dockerCfg := dockercfg.LoadDefaultConfigFile(console.Stderr(ctx))

		addr := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", arn.AccountID, *region)

		dockerCfg.AuthConfigs[addr] = types.AuthConfig{
			ServerAddress: addr,
			Username:      parts[0],
			Password:      parts[1],
		}

		filename := dockerCfg.Filename

		if *temp {
			tmpDir, err := dirs.CreateUserTempDir("dockerconfig", "*")
			if err != nil {
				return fnerrors.Newf("failed to create temp dir: %w", err)
			}

			tmpFile, err := os.OpenFile(filepath.Join(tmpDir, dockerCfgName), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return fnerrors.Newf("failed to create temp file: %w", err)
			}

			if err := dockerCfg.SaveToWriter(tmpFile); err != nil {
				return fnerrors.Newf("failed to save config: %w", err)
			}

			if err := tmpFile.Close(); err != nil {
				return fnerrors.Newf("failed to close docker config: %w", err)
			}

			filename = tmpFile.Name()
		} else {
			if err := dockerCfg.Save(); err != nil {
				return fnerrors.Newf("failed to save config: %w", err)
			}
		}

		fmt.Fprintf(console.Stderr(ctx), "Added ECR credentials to %s.\n", filename)

		if *outputPath != "" {
			if err := os.WriteFile(*outputPath, []byte(filename), 0644); err != nil {
				return fnerrors.Newf("failed to write %q: %w", *outputPath, err)
			}
		}

		return nil
	})
}

type staticToken struct {
	idToken string
}

func (st staticToken) GetIdentityToken() ([]byte, error) {
	return []byte(st.idToken), nil
}
