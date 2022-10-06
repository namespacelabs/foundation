// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package iam

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/internal/providers/aws"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

func RegisterGraphHandlers() {
	execution.RegisterFuncs(execution.Funcs[*OpEnsureRole]{
		Aliases: []string{"type.googleapis.com/foundation.providers.aws.iam.OpEnsureRole"},
		Handle: func(ctx context.Context, def *schema.SerializedInvocation, m *OpEnsureRole) (*execution.HandleResult, error) {
			if m.AssumeRolePolicyJson == "" || m.RoleName == "" {
				return nil, fnerrors.BadInputError("both role_name and assume_role_policy_json are required")
			}

			config, err := execution.Get(ctx, execution.ConfigurationInjection)
			if err != nil {
				return nil, err
			}

			sesh, err := awsprovider.MustConfiguredSession(ctx, config)
			if err != nil {
				return nil, err
			}

			input := &iam.CreateRoleInput{
				RoleName:                 &m.RoleName,
				AssumeRolePolicyDocument: &m.AssumeRolePolicyJson,
			}

			if m.Description != "" {
				input.Description = &m.Description
			}

			addTags(&input.Tags, m.Tag, m.ForServer)

			iamcli := iam.NewFromConfig(sesh.Config())

			if _, err := iamcli.CreateRole(ctx, input); err != nil {
				var e *types.EntityAlreadyExistsException
				if errors.As(err, &e) {
					if _, err := iamcli.TagRole(ctx, &iam.TagRoleInput{Tags: input.Tags, RoleName: input.RoleName}); err != nil {
						return nil, fnerrors.InvocationError("IAM role already existed, and failed to update its tags: %w", err)
					}

					if _, err := iamcli.UpdateAssumeRolePolicy(ctx, &iam.UpdateAssumeRolePolicyInput{
						RoleName:       &m.RoleName,
						PolicyDocument: &m.AssumeRolePolicyJson,
					}); err != nil {
						return nil, fnerrors.InvocationError("IAM role already existed, and failed to update its policy: %w", err)
					}
				} else {
					return nil, fnerrors.InvocationError("failed to create IAM role: %w", err)
				}
			}

			return nil, nil
		},

		PlanOrder: func(*OpEnsureRole) (*schema.ScheduleOrder, error) {
			return &schema.ScheduleOrder{
				SchedCategory: []string{"aws:iam:ensure-role"},
			}, nil
		},
	})

	execution.RegisterFuncs(execution.Funcs[*OpAssociatePolicy]{
		Aliases: []string{"type.googleapis.com/foundation.providers.aws.iam.OpAssociatePolicy"},
		Handle: func(ctx context.Context, def *schema.SerializedInvocation, m *OpAssociatePolicy) (*execution.HandleResult, error) {
			if m.PolicyJson == "" || m.PolicyName == "" || m.RoleName == "" {
				return nil, fnerrors.BadInputError("all of `role_name` and `policy_name` and `policy_json` are required")
			}

			config, err := execution.Get(ctx, execution.ConfigurationInjection)
			if err != nil {
				return nil, err
			}

			sesh, err := awsprovider.MustConfiguredSession(ctx, config)
			if err != nil {
				return nil, err
			}

			input := &iam.PutRolePolicyInput{
				PolicyName:     &m.PolicyName,
				PolicyDocument: &m.PolicyJson,
				RoleName:       &m.RoleName,
			}

			iamcli := iam.NewFromConfig(sesh.Config())

			if _, err := iamcli.PutRolePolicy(ctx, input); err != nil {
				var e *types.EntityAlreadyExistsException
				if !errors.As(err, &e) {
					return nil, fnerrors.InvocationError("failed to attach policy to role: %w", err)
				}
			}

			return nil, nil
		},

		PlanOrder: func(*OpAssociatePolicy) (*schema.ScheduleOrder, error) {
			// Run policy associations after we create roles.
			return &schema.ScheduleOrder{
				SchedAfterCategory: []string{"aws:iam:ensure-role"},
			}, nil
		},
	})
}

type tagLike interface {
	GetKey() string
	GetValue() string
}

func addTags[V tagLike](tags *[]types.Tag, userSpecified []V, srv *schema.Server) {
	for _, t := range userSpecified {
		*tags = append(*tags, tag(t.GetKey(), t.GetValue()))
	}

	if srv != nil {
		*tags = append(*tags,
			tag("alpha.foundation.namespacelabs.com/server-id", srv.Id),
			tag("alpha.foundation.namespacelabs.com/server-package-name", srv.PackageName),
		)
	}
}

func tag(k, v string) types.Tag {
	return types.Tag{Key: &v, Value: &v}
}
