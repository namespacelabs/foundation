// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/providers/aws/eks"
	fniam "namespacelabs.dev/foundation/providers/aws/iam"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	if err := configure.RunServer(context.Background(), func(sr grpc.ServiceRegistrar) {
		h := configure.NewHandlers()
		h.Any().HandleStack(provisionHook{})

		protocol.RegisterInvocationServiceServer(sr, h.ServiceHandler())
	}); err != nil {
		log.Fatal(err)
	}
}

type provisionHook struct{}

func (provisionHook) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return fnerrors.BadInputError("universe/aws/irsa only supports kubernetes")
	}

	serviceAccount := &kubedef.ServiceAccountDetails{}
	if err := r.UnpackInput(serviceAccount); err != nil {
		return err
	}

	eksCluster := &eks.EKSCluster{}
	if ok, err := r.CheckUnpackInput(eksCluster); err != nil {
		return err
	} else if !ok {
		return nil
	}

	eksServerDetails := &eks.EKSServerDetails{}
	if err := r.UnpackInput(eksServerDetails); err != nil {
		return err
	}

	if eksCluster.Arn == "" {
		return fnerrors.InternalError("eks_cluster.arn is missing")
	}

	if eksCluster.OidcIssuer == "" {
		return fnerrors.BadInputError("eks_cluster does not have an OIDC issuer assigned")
	}

	clusterArn, err := arn.Parse(eksCluster.Arn)
	if err != nil {
		return err
	}

	if clusterArn.AccountID == "" {
		return fnerrors.BadInputError("eks_cluster.arn is missing account id")
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			ServiceAccount: serviceAccount.ServiceAccountName,
			ServiceAccountAnnotation: []*kubedef.SpecExtension_Annotation{
				{Key: "eks.amazonaws.com/role-arn", Value: fmt.Sprintf("arn:aws:iam::%s:role/%s",
					clusterArn.AccountID, eksServerDetails.ComputedIamRoleName)},
			},
		},
	})

	oidcProvider := strings.TrimPrefix(eksCluster.OidcIssuer, "https://")

	namespace := kubetool.FromRequest(r).Namespace

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
		return fnerrors.InternalError("failed to serialize policy: %w", err)
	}

	out.Invocations = append(out.Invocations, makeRole{
		&fniam.OpEnsureRole{
			RoleName: eksServerDetails.ComputedIamRoleName,
			Description: fmt.Sprintf("Foundation-managed IAM role for service account %s/%s in EKS cluster %s",
				namespace, serviceAccount, eksCluster.Name),
			AssumeRolePolicyJson: string(policyBytes),
			ForServer:            r.Focus.Server,
		},
	})

	return nil
}

func (provisionHook) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return fnerrors.BadInputError("universe/aws/irsa only supports kubernetes")
	}

	return nil
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
