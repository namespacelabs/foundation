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
	fniam "namespacelabs.dev/foundation/providers/aws/iam"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/provision/tool/protocol"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
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

	sysInfo := &client.SystemInfo{}
	if err := r.UnpackInput(sysInfo); err != nil {
		return err
	}

	// If we're not deploying to EKS, skip IAM role.
	if sysInfo.DetectedDistribution != "eks" || sysInfo.EksCluster == nil {
		return nil
	}

	if sysInfo.EksCluster.Arn == "" {
		return fnerrors.InternalError("eks_cluster.arn is missing")
	}

	if sysInfo.EksCluster.OidcIssuer == "" {
		return fnerrors.BadInputError("eks_cluster does not have an OIDC issuer assigned")
	}

	clusterArn, err := arn.Parse(sysInfo.EksCluster.Arn)
	if err != nil {
		return err
	}

	if clusterArn.AccountID == "" {
		return fnerrors.BadInputError("eks_cluster.arn is missing account id")
	}

	serviceAccount := kubedef.MakeDeploymentId(r.Focus.Server) // XXX this should be an input.

	// IAM roles have a maximum length of 64.
	iamRole := fmt.Sprintf("foundation-%s-%s-%s-%s",
		sysInfo.EksCluster.Name, r.Env.Name, r.Focus.Server.Name, r.Focus.Server.Id)
	if len(iamRole) > 64 {
		return fnerrors.InternalError("generated a role name that is too long (%d): %s", len(iamRole), iamRole)
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		For: schema.PackageName(r.Focus.Server.PackageName),
		With: &kubedef.SpecExtension{
			ServiceAccount:       serviceAccount,
			EnsureServiceAccount: true,
			ServiceAccountAnnotation: []*kubedef.SpecExtension_Annotation{
				{Key: "eks.amazonaws.com/role-arn", Value: fmt.Sprintf("arn:aws:iam::%s:role/%s", clusterArn.AccountID, iamRole)},
			},
		},
	})

	oidcProvider := strings.TrimPrefix(sysInfo.EksCluster.OidcIssuer, "https://")

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
			Name: iamRole,
			Description: fmt.Sprintf("Foundation-managed IAM role for service account %s/%s in EKS cluster %s",
				namespace, serviceAccount, sysInfo.EksCluster.Name),
			AssumeRolePolicyJson: string(policyBytes),
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
		Description: fmt.Sprintf("AWS IAM role %s", m.Name),
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
