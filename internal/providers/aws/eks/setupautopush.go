// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package eks

import (
	"encoding/json"
	"fmt"

	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/execution/defs"
	fneks "namespacelabs.dev/foundation/universe/aws/eks"
	fniam "namespacelabs.dev/foundation/universe/aws/iam"
)

// AWS account ID for our hosted CI pipeline.
// TODO consider making this configurable for self-hosted pipelines.
const ciAcc = 960279036429

var ciRoles = []string{
	// "fntest", // Removed for now. Tests don't run in staging cluster.
	"fnplandeploy",
	"fndeploy",
}

func SetupAutopush(eksCluster *fneks.EKSCluster, iamRole string, roleArn string) ([]defs.MakeDefinition, error) {
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

	out = append(out, defs.Static(fmt.Sprintf("AWS IAM role %s", iamRole), &fniam.OpEnsureRole{
		RoleName:             iamRole,
		Description:          fmt.Sprintf("Namespace-managed IAM role for autopush deployment in EKS cluster %s", eksCluster.Name),
		AssumeRolePolicyJson: string(policyBytes),
	}))

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

	clusterRole := fmt.Sprintf("ns:%s-clusterrole", iamRole)
	out = append(out, kubedef.Apply{
		Description: "Admin Cluster Role",
		Resource: applyrbacv1.ClusterRole(clusterRole).
			WithAnnotations(kubedef.BaseAnnotations()).
			WithRules(
				applyrbacv1.PolicyRule().WithAPIGroups("*").WithResources("*").
					WithVerbs("apply", "create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"),
				applyrbacv1.PolicyRule().WithNonResourceURLs("*").
					WithVerbs("apply", "create", "delete", "deletecollection", "get", "list", "patch", "update", "watch")),
	})

	group := fmt.Sprintf("ns:%s-group", iamRole)
	out = append(out, kubedef.Apply{
		Description: "Admin Cluster Role Binding",
		Resource: applyrbacv1.ClusterRoleBinding(fmt.Sprintf("ns:%s-binding", iamRole)).
			WithAnnotations(kubedef.BaseAnnotations()).
			WithRoleRef(applyrbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(clusterRole)).
			WithSubjects(applyrbacv1.Subject().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("Group").
				WithName(group)),
	})

	out = append(out, defs.Static(fmt.Sprintf("AWS auth for %s", iamRole), &OpEnsureAwsAuth{
		Username: iamRole,
		Rolearn:  roleArn,
		Group:    []string{group},
	}))

	return out, nil
}
