// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package eks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution/defs"
	fniam "namespacelabs.dev/foundation/universe/aws/iam"
)

type IrsaResult struct {
	Invocations []defs.MakeDefinition
	Extensions  []defs.MakeExtension
}

func PrepareIrsa(eksCluster *EKSCluster, iamRole, namespace, serviceAccount string, srv *schema.Server) (*IrsaResult, error) {
	if eksCluster.Arn == "" {
		return nil, fnerrors.InternalError("eks_cluster.arn is missing")
	}

	if !eksCluster.HasOidcProvider {
		return nil, fnerrors.UsageError(
			"See https://docs.namespace.so/guides/production/#no-oidc-provider-for-a-cluster for ways to solve this issue",
			"eks_cluster does not have an OIDC issuer assigned: %s", eksCluster.OidcIssuer)
	}

	clusterArn, err := arn.Parse(eksCluster.Arn)
	if err != nil {
		return nil, err
	}

	if clusterArn.AccountID == "" {
		return nil, fnerrors.BadInputError("eks_cluster.arn is missing account id")
	}

	var out IrsaResult
	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			ServiceAccount: serviceAccount,
			ServiceAccountAnnotation: []*kubedef.SpecExtension_Annotation{
				{Key: "eks.amazonaws.com/role-arn", Value: fmt.Sprintf("arn:aws:iam::%s:role/%s",
					clusterArn.AccountID, iamRole)},
			},
		},
	})

	oidcProvider := strings.TrimPrefix(eksCluster.OidcIssuer, "https://")

	policy := fniam.PolicyDocument{
		Version: "2012-10-17",
		Statement: []fniam.StatementEntry{
			{
				Effect: "Allow",
				Principal: &fniam.Principal{
					Federated: fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", clusterArn.AccountID, oidcProvider),
				},
				Action: []string{"sts:AssumeRoleWithWebIdentity"},
				Condition: &fniam.Condition{
					StringEquals: []fniam.Condition_KeyValue{
						{Key: fmt.Sprintf("%s:aud", oidcProvider), Value: "sts.amazonaws.com"},
						{Key: fmt.Sprintf("%s:sub", oidcProvider), Value: fmt.Sprintf("system:serviceaccount:%s:%s", namespace, serviceAccount)},
					},
				},
			},
		},
	}

	policyBytes, err := json.Marshal(policy)
	if err != nil {
		return nil, fnerrors.InternalError("failed to serialize policy: %w", err)
	}

	out.Invocations = append(out.Invocations, defs.Static(fmt.Sprintf("AWS IAM role %s", iamRole), &fniam.OpEnsureRole{
		RoleName: iamRole,
		Description: fmt.Sprintf("Namespace-managed IAM role for service account %s/%s in EKS cluster %s",
			namespace, serviceAccount, eksCluster.Name),
		AssumeRolePolicyJson: string(policyBytes),
		ForServer:            srv,
	}))

	return &out, nil
}
