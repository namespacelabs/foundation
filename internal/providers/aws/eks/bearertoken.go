// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package eks

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sts"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func ComputeBearerToken(ctx context.Context, s *Session, clusterName string) (Token, error) {
	g := generator{
		client: sts.NewPresignClient(sts.NewFromConfig(s.sesh.Config())),
	}

	return g.GetWithSTS(ctx, clusterName)
}

// Adapted from https://github.com/weaveworks/eksctl/blob/e7de320db622068ce1c7fb5d9d19fe8b4ddb22cd/pkg/eks/generator.go#L53
// To reduce number of dependencies.

// Token is generated and used by Kubernetes client-go to authenticate with a Kubernetes cluster.
type Token struct {
	Token      string
	Expiration time.Time
}

const (
	clusterIDHeader        = "x-k8s-aws-id"
	presignedURLExpiration = 15 * time.Minute
	v1Prefix               = "k8s-aws-v1."
)

type generator struct {
	client *sts.PresignClient
}

// GetWithSTS returns a token valid for clusterID using the given STS client.
// This implementation follows the steps outlined here:
// https://github.com/kubernetes-sigs/aws-iam-authenticator#api-authorization-from-outside-a-cluster
// We either add this implementation or have to maintain two versions of STS since aws-iam-authenticator is
// not switching over to aws-go-sdk-v2.
func (g generator) GetWithSTS(ctx context.Context, clusterID string) (Token, error) {
	// generate a sts:GetCallerIdentity request and add our custom cluster ID header
	presignedURLRequest, err := g.client.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{}, func(presignOptions *sts.PresignOptions) {
		presignOptions.ClientOptions = append(presignOptions.ClientOptions, g.appendPresignHeaderValuesFunc(clusterID))
	})
	if err != nil {
		return Token{}, fmt.Errorf("failed to presign caller identity: %w", err)
	}

	// Set token expiration to 1 minute before the presigned URL expires for some cushion
	tokenExpiration := time.Now().Local().Add(presignedURLExpiration - 1*time.Minute)
	// Add the token with k8s-aws-v1. prefix.
	return Token{v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(presignedURLRequest.URL)), tokenExpiration}, nil
}

func (g generator) appendPresignHeaderValuesFunc(clusterID string) func(stsOptions *sts.Options) {
	return func(stsOptions *sts.Options) {
		// Add clusterId Header
		stsOptions.APIOptions = append(stsOptions.APIOptions, smithyhttp.SetHeaderValue(clusterIDHeader, clusterID))
		// Add X-Amz-Expires query param
		stsOptions.APIOptions = append(stsOptions.APIOptions, smithyhttp.SetHeaderValue("X-Amz-Expires", "60"))
	}
}
