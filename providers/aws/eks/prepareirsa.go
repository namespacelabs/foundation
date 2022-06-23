// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package eks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/engine/ops/defs"
	"namespacelabs.dev/foundation/internal/fnerrors"
	fniam "namespacelabs.dev/foundation/providers/aws/iam"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

type IrsaResult struct {
	Invocations []defs.MakeDefinition
	Extensions  []defs.MakeExtension
}

func PrepareIrsa(eksCluster *EKSCluster, iamRole, namespace, serviceAccount string, srv *schema.Server) (*IrsaResult, error) {
	if eksCluster.Arn == "" {
		return nil, fnerrors.InternalError("eks_cluster.arn is missing")
	}

	if eksCluster.OidcIssuer == "" {
		return nil, fnerrors.BadInputError("eks_cluster does not have an OIDC issuer assigned")
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

	out.Invocations = append(out.Invocations, makeRole{
		&fniam.OpEnsureRole{
			RoleName: iamRole,
			Description: fmt.Sprintf("Namespace-managed IAM role for service account %s/%s in EKS cluster %s",
				namespace, serviceAccount, eksCluster.Name),
			AssumeRolePolicyJson: string(policyBytes),
			ForServer:            srv,
		},
	})

	return &out, nil
}

type makeRole struct {
	*fniam.OpEnsureRole
}

func (m makeRole) ToDefinition(scope ...schema.PackageName) (*schema.SerializedInvocation, error) {
	packed, err := anypb.New(m.OpEnsureRole)
	if err != nil {
		return nil, err
	}

	return &schema.SerializedInvocation{
		Description: fmt.Sprintf("AWS IAM role %s", m.RoleName),
		Impl:        packed,
		Scope:       schema.Strs(scope...),
	}, nil
}
