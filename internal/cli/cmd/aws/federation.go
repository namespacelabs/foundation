// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package aws

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	idTokenVersion = 1
)

func newAssumeRoleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assume-role",
		Short: "Uses Workload Federation to assume a particular AWS role.",
		Args:  cobra.NoArgs,
	}

	roleArn := cmd.Flags().String("role_arn", "", "The ARN of the role to assume.")
	awsProfile := cmd.Flags().String("aws_profile", "", "Use the specified AWS profile.")
	envFile := cmd.Flags().String("write_env", "", "Instead of outputting, write the environment variables to the specified file.")
	duration := cmd.Flags().Duration("duration", time.Hour, "For how long the resulting AWS credentials should be valid for.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *roleArn == "" {
			return fnerrors.New("--role_arn is required")
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

		t := time.Now()
		resp, err := fnapi.IssueIdToken(ctx, "sts.amazonaws.com", idTokenVersion)
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
		arole, err := client.AssumeRoleWithWebIdentity(ctx, &sts.AssumeRoleWithWebIdentityInput{
			RoleArn:          aws.String(*roleArn),
			RoleSessionName:  aws.String(sessionName),
			WebIdentityToken: aws.String(resp.IdToken),
			DurationSeconds:  aws.Int32(int32(duration.Seconds())),
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Obtained credentials from AWS (took=%v).\n", time.Since(t))

		fmt.Fprintf(out, "export AWS_ACCESS_KEY_ID=%s\n", *arole.Credentials.AccessKeyId)
		fmt.Fprintf(out, "export AWS_SECRET_ACCESS_KEY=%s\n", *arole.Credentials.SecretAccessKey)
		fmt.Fprintf(out, "export AWS_SESSION_TOKEN=%s\n", *arole.Credentials.SessionToken)

		if *envFile != "" {
			fmt.Fprintf(console.Stderr(ctx), "Wrote %s.\n", *envFile)
		}

		return nil
	})
}

func newSTSClient(ctx context.Context, awsProfile string) (*sts.Client, error) {
	var opts []func(*config.LoadOptions) error

	opts = append(opts, config.WithRegion("us-east-1"))

	if awsProfile != "" {
		opts = append(opts, config.WithSharedConfigProfile(awsProfile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return sts.NewFromConfig(cfg), nil
}
