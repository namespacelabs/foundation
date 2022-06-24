// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package aws

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"google.golang.org/protobuf/encoding/prototext"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
)

const identityTokenEnv = "AWS_WEB_IDENTITY_TOKEN_FILE"

func hasWebIdentityEnvVar() bool {
	// Check if we run inside an AWS cluster with a configured IAM role.
	token := os.Getenv(identityTokenEnv)
	return token != ""
}

func ConfiguredSession(ctx context.Context, devHost *schema.DevHost, selector devhost.Selector) (*Session, error) {
	return configuredSession(ctx, devHost, selector)
}

func configuredSession(ctx context.Context, devHost *schema.DevHost, selector devhost.Selector) (*Session, error) {
	makeSession, conf, err := innerSession(ctx, devHost, selector)
	if err != nil {
		return nil, err
	}

	if makeSession == nil {
		return nil, nil
	}

	session, err := makeSession()
	if err != nil {
		return nil, err
	}

	if conf.AssumeRoleArn != "" {
		stsclient := sts.NewFromConfig(session)
		assumedSession, err := makeSession(config.WithCredentialsProvider(
			aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(stsclient, conf.AssumeRoleArn))))
		if err != nil {
			return nil, err
		}
		return &Session{aws: assumedSession, conf: conf}, nil
	}

	return &Session{aws: session, conf: conf}, nil
}

func configuration(devHost *schema.DevHost, selector devhost.Selector) *Conf {
	conf := &Conf{}
	if selector == nil || !selector.Select(devHost).Get(conf) {
		if hasWebIdentityEnvVar() {
			// TODO: remove profile?
			return &Conf{UseInjectedWebIdentity: true}
		}
		return nil
	}

	return conf
}

func innerSession(ctx context.Context, devHost *schema.DevHost, selector devhost.Selector) (func(...func(*config.LoadOptions) error) (aws.Config, error), *Conf, error) {
	conf := configuration(devHost, selector)
	if conf == nil {
		return nil, nil, nil
	}

	if conf.GetUseInjectedWebIdentity() {
		if !hasWebIdentityEnvVar() {
			return nil, nil, fnerrors.BadInputError("aws: use_injected_web_identity was specified but no %q env var found", identityTokenEnv)
		}

		return func(opts ...func(*config.LoadOptions) error) (aws.Config, error) {
			return config.LoadDefaultConfig(ctx, opts...)
		}, conf, nil
	}

	return func(opts ...func(*config.LoadOptions) error) (aws.Config, error) {
		return config.LoadDefaultConfig(ctx, append(opts, config.WithSharedConfigProfile(conf.Profile))...)
	}, conf, nil
}

type Session struct {
	aws  aws.Config
	conf *Conf
}

func (s *Session) Config() aws.Config { return s.aws }
func (s *Session) CacheKey() string   { return prototext.Format(s.conf) }
func (s *Session) RefreshUsage() string {
	if s.conf.UseInjectedWebIdentity {
		return ""
	}

	if s.conf.Profile == "" {
		return "Try running `aws sso login`."
	}

	return fmt.Sprintf("Try running `aws --profile %s sso login`.", s.conf.Profile)
}

// MustConfiguredSession also returns a cache key.
func MustConfiguredSession(ctx context.Context, devHost *schema.DevHost, selector devhost.Selector) (*Session, error) {
	session, err := configuredSession(ctx, devHost, selector)
	if err != nil {
		return nil, err
	}

	if session == nil {
		return nil, fnerrors.UsageError("Run `ns prepare`.", "Namespace has not been configured to access AWS.")
	}

	return session, nil
}
