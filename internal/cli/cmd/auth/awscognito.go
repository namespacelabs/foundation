package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func newExchangeAwsCognitoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "exchange-aws-cognito-token",
		Short:  "Generate a Namespace Cloud token from a AWS Cognito JWT.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	awsRegion := cmd.Flags().String("aws_region", "", "The AWS region to connect to.")
	awsProfile := cmd.Flags().String("aws_profile", "", "Use the specified AWS profile.")
	identityPool := cmd.Flags().String("identity_pool", "", "UUID of the identity pool.")
	ensuredDuration := cmd.Flags().Duration("ensure", 0, "If the current token is still valid for this duration, do nothing. Otherwise fetch a new token.")
	tenantId := cmd.Flags().String("tenant_id", "", "What tenant to authenticate.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *awsRegion == "" {
			return fnerrors.New("--aws_region is required")
		}

		if *identityPool == "" {
			return fnerrors.New("--identity_pool is required")
		}

		if *tenantId == "" {
			return fnerrors.New("--tenant_id is required")
		}

		actual := strings.TrimPrefix(*identityPool, *awsRegion+":")

		if *ensuredDuration > 0 {
			var reauth *fnerrors.ReauthErr
			if err := auth.EnsureTokenValidAt(ctx, time.Now().Add(*ensuredDuration)); err == nil {
				// Token is valid for entire duration.
				return nil
			} else if !errors.As(err, &reauth) {
				// failed to load token
				return err
			}
		}

		client, err := newCognitoClient(ctx, *awsProfile)
		if err != nil {
			return err
		}

		resp, err := client.GetOpenIdTokenForDeveloperIdentity(ctx, &cognitoidentity.GetOpenIdTokenForDeveloperIdentityInput{
			IdentityPoolId: aws.String(fmt.Sprintf("%s:%s", *awsRegion, actual)),
			Logins: map[string]string{
				"namespace.so": *tenantId,
			},
		}, func(opts *cognitoidentity.Options) {
			opts.Region = *awsRegion
		})
		if err != nil {
			return err
		}

		fmt.Fprintf(console.Stderr(ctx), "Obtained token from AWS Cognito.\n")

		token, err := fnapi.ExchangeAWSCognitoJWT(ctx, *tenantId, *resp.Token)
		if err != nil {
			return err
		}

		if token.Tenant != nil {
			if token.Tenant.Name != "" {
				fmt.Fprintf(console.Stdout(ctx), "You are now logged into workspace %q, have a nice day.\n", token.Tenant.Name)
			}
			if token.Tenant.AppUrl != "" {
				fmt.Fprintf(console.Stdout(ctx), "You can inspect you clusters at %s\n", token.Tenant.AppUrl)
			}
		}

		return auth.StoreTenantToken(token.TenantToken)
	})
}

func newCognitoClient(ctx context.Context, awsProfile string) (*cognitoidentity.Client, error) {
	var opts []func(*config.LoadOptions) error

	if awsProfile != "" {
		opts = append(opts, config.WithSharedConfigProfile(awsProfile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}

	return cognitoidentity.NewFromConfig(cfg), nil
}

func newTrustAwsCognitoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "trust-aws-cognito-identity-pool",
		Short:  "Trust a AWS Cognito Identity pool.",
		Args:   cobra.NoArgs,
		Hidden: true,
	}

	awsRegion := cmd.Flags().String("aws_region", "", "The AWS region to connect to.")
	identityPool := cmd.Flags().String("identity_pool", "", "UUID of the identity pool.")
	tenantId := cmd.Flags().String("tenant_id", "", "What tenant to authenticate.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		if *awsRegion == "" {
			return fnerrors.New("--aws_region is required")
		}

		if *identityPool == "" {
			return fnerrors.New("--identity_pool is required")
		}

		if *tenantId == "" {
			return fnerrors.New("--tenant_id is required")
		}

		actual := strings.TrimPrefix(*identityPool, *awsRegion+":")
		pool := fmt.Sprintf("%s:%s", *awsRegion, actual)

		return fnapi.TrustAWSCognitoJWT(ctx, *tenantId, pool, "namespace.so")
	})
}
