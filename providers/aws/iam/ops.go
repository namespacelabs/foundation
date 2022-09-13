// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package iam

import (
	"context"
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	awsprovider "namespacelabs.dev/foundation/providers/aws"
	"namespacelabs.dev/foundation/schema"
)

func RegisterGraphHandlers() {
	ops.RegisterFunc(func(ctx context.Context, def *schema.SerializedInvocation, m *OpEnsureRole) (*ops.HandleResult, error) {
		if m.AssumeRolePolicyJson == "" || m.RoleName == "" {
			return nil, fnerrors.BadInputError("both role_name and assume_role_policy_json are required")
		}

		config, err := ops.Get(ctx, ops.ConfigurationInjection)
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
	})

	ops.RegisterFunc(func(ctx context.Context, def *schema.SerializedInvocation, m *OpAssociatePolicy) (*ops.HandleResult, error) {
		if m.PolicyJson == "" || m.PolicyName == "" || m.RoleName == "" {
			return nil, fnerrors.BadInputError("all of `role_name` and `policy_name` and `policy_json` are required")
		}

		config, err := ops.Get(ctx, ops.ConfigurationInjection)
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
	})

	// Run policy associations after we create roles.
	ops.RunAfter(&OpEnsureRole{}, &OpAssociatePolicy{})
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
