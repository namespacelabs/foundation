// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"encoding/json"
	"fmt"

	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	fniam "namespacelabs.dev/foundation/providers/aws/iam"
)

const ciAcc = 960279036429

var ciRoles = [...]string{"fntest", "fnplandeploy", "fndeploy"}

func SetupAutopush(eksCluster *EKSCluster, iamRole string) ([]defs.MakeDefinition, error) {
	var out []defs.MakeDefinition

	policy := fniam.PolicyDocument{
		Version: "2012-10-17",
	}

	for _, role := range ciRoles {
		policy.Statement = append(policy.Statement, fniam.StatementEntry{
			Effect: "Allow",
			Principal: &fniam.Principal{
				AWS: fmt.Sprintf("arn:aws:iam::%d:role/%s", ciAcc, role),
			},
			Action: []string{"sts:AssumeRole"},
		})
	}

	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return nil, fnerrors.InternalError("failed to serialize policy: %w", err)
	}

	out = append(out, makeRole{
		&fniam.OpEnsureRole{
			RoleName:             iamRole,
			Description:          fmt.Sprintf("Namespace-managed IAM role for autopush deployment in EKS cluster %s", eksCluster.Name),
			AssumeRolePolicyJson: string(policyBytes),
		},
	})

	policy = fniam.PolicyDocument{
		Version: "2012-10-17",
		Statement: []fniam.StatementEntry{
			{
				Effect:   "Allow",
				Action:   []string{"ecr:*"},
				Resource: []string{"*"},
			},
			{
				Effect:   "Allow",
				Action:   []string{"eks:*"},
				Resource: []string{eksCluster.Arn},
			},
			{
				Effect:   "Allow",
				Action:   []string{"iam:*"},
				Resource: []string{"*"},
			},
		},
	}

	policyBytes, err = json.Marshal(policy)
	if err != nil {
		return nil, fnerrors.InternalError("failed to serialize policy: %w", err)
	}

	associate := &fniam.OpAssociatePolicy{
		RoleName:   iamRole,
		PolicyName: "namespace-ci-aws-access",
		PolicyJson: string(policyBytes),
	}
	out = append(out, defs.Static("Namespace CI AWS Access IAM Policy", associate))

	return out, nil
}
