// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"bytes"
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
		protocol.RegisterInvocationServiceServer(sr, configure.ProtocolHandler{Handlers: configure.HandlerCompat{Tool: tool{}}})
	}); err != nil {
		log.Fatal(err)
	}
}

type tool struct{}

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
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
		For: schema.PackageName(r.Focus.Server.PackageName),
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

	policy := PolicyDocument{
		Version: "2012-10-17",
		Statement: []StatementEntry{
			{
				Effect: "Allow",
				Principal: Principal{
					Federated: fmt.Sprintf("arn:aws:iam::%s:oidc-provider/%s", clusterArn.AccountID, oidcProvider),
				},
				Action: []string{"sts:AssumeRoleWithWebIdentity"},
				Condition: Condition{
					StringEquals: []kv{
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

	out.Definitions = append(out.Definitions, makeRole{
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

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	if r.Env.Runtime != "kubernetes" {
		return fnerrors.BadInputError("universe/aws/irsa only supports kubernetes")
	}

	return nil
}

type makeRole struct {
	*fniam.OpEnsureRole
}

func (m makeRole) ToDefinition(scope ...schema.PackageName) (*schema.Definition, error) {
	packed, err := anypb.New(m.OpEnsureRole)
	if err != nil {
		return nil, err
	}

	return &schema.Definition{
		Description: fmt.Sprintf("AWS IAM role %s", m.RoleName),
		Impl:        packed,
		Scope:       schema.Strs(scope...),
	}, nil
}

type PolicyDocument struct {
	Version   string
	Statement []StatementEntry
}

type StatementEntry struct {
	Effect    string
	Principal Principal
	Action    []string
	Condition Condition
}

type Principal struct {
	Federated string
}

type Condition struct {
	StringEquals []kv
}

type kv struct {
	Key   string
	Value string
}

func (c Condition) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "{%q:{", "StringEquals")
	for k, kv := range c.StringEquals {
		fmt.Fprintf(&b, "%q:%q", kv.Key, kv.Value)
		if k < len(c.StringEquals)-1 {
			fmt.Fprintf(&b, ",")
		}
	}
	fmt.Fprintf(&b, "}}")
	return b.Bytes(), nil
}
