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
	ops.RegisterFunc(func(ctx context.Context, env ops.Environment, def *schema.Definition, m *OpEnsureRole) (*ops.DispatcherResult, error) {
		if m.AssumeRolePolicyJson == "" || m.RoleName == "" {
			return nil, fnerrors.BadInputError("both role_name and assume_role_policy_json are required")
		}

		sesh, _, err := awsprovider.ConfiguredSession(ctx, env.DevHost(), env.Proto())
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

		for _, tag := range m.Tag {
			key := tag.Key
			value := tag.Value
			input.Tags = append(input.Tags, types.Tag{
				Key: &key, Value: &value,
			})
		}

		iamcli := iam.NewFromConfig(sesh)

		if _, err := iamcli.CreateRole(ctx, input); err != nil {
			var e *types.EntityAlreadyExistsException
			if errors.As(err, &e) {
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
}
