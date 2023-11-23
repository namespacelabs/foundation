// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"time"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

const AdminScope = "admin"

type ExchangeGithubTokenRequest struct {
	GithubToken string `json:"github_token,omitempty"`
}

type ExchangeGithubTokenResponse struct {
	TenantToken string  `json:"tenant_token,omitempty"`
	Tenant      *Tenant `json:"tenant,omitempty"`
}

type ExchangeCircleciTokenRequest struct {
	CircleciToken string `json:"circleci_token,omitempty"`
}

type ExchangeCircleciTokenResponse struct {
	TenantToken string  `json:"tenant_token,omitempty"`
	Tenant      *Tenant `json:"tenant,omitempty"`
}

type ExchangeAWSCognitoJWTRequest struct {
	TenantId        string `json:"tenant_id,omitempty"`
	AwsCognitoToken string `json:"aws_cognito_token,omitempty"`
}

type ExchangeOIDCTokenRequest struct {
	TenantId  string `json:"tenant_id,omitempty"`
	OidcToken string `json:"oidc_token,omitempty"`
}

type ExchangeTokenResponse struct {
	TenantToken string  `json:"tenant_token,omitempty"`
	Tenant      *Tenant `json:"tenant,omitempty"`
}

type TrustAWSCognitoIdentityPoolRequest struct {
	AwsCognitoIdentityPool string `json:"aws_cognito_identity_pool,omitempty"` // E.g. eu-central-1:56388dff-961f-42d4-a2ac-6ad118eb7799
	IdentityProvider       string `json:"identity_provider,omitempty"`         // E.g. namespace.so
}

type IssueIdTokenRequest struct {
	Audience string `json:"audience,omitempty"`
	Version  int    `json:"version,omitempty"`
}

type IssueIdTokenResponse struct {
	IdToken string `json:"id_token,omitempty"`
}

type IssueIngressAccessTokenRequest struct {
	InstanceId string `json:"instance_id,omitempty"`
}

type IssueIngressAccessTokenResponse struct {
	IngressAccessToken string `json:"ingress_access_token,omitempty"`
}

type IssueDevelopmentTokenResponse struct {
	DevelopmentToken string `json:"development_token,omitempty"`
}

type Tenant struct {
	TenantId      string `json:"tenant_id,omitempty"`
	Name          string `json:"name,omitempty"`
	AppUrl        string `json:"app_url,omitempty"`
	PrimaryRegion string `json:"primary_region,omitempty"`
}

func ExchangeGithubToken(ctx context.Context, jwt string) (ExchangeGithubTokenResponse, error) {
	req := ExchangeGithubTokenRequest{GithubToken: jwt}

	var res ExchangeGithubTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeGithubToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeGithubTokenResponse{}, err
	}

	return res, nil
}

func ExchangeCircleciToken(ctx context.Context, token string) (ExchangeCircleciTokenResponse, error) {
	req := ExchangeCircleciTokenRequest{CircleciToken: token}

	var res ExchangeCircleciTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeCircleciToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeCircleciTokenResponse{}, err
	}

	return res, nil
}

func ExchangeOIDCToken(ctx context.Context, tenantID, token string) (ExchangeTokenResponse, error) {
	req := ExchangeOIDCTokenRequest{TenantId: tenantID, OidcToken: token}

	var res ExchangeTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeOIDCToken", req, DecodeJSONResponse(&res)); err != nil {
		return ExchangeTokenResponse{}, err
	}

	return res, nil
}

func ExchangeAWSCognitoJWT(ctx context.Context, tenantID, token string) (ExchangeTokenResponse, error) {
	req := ExchangeAWSCognitoJWTRequest{TenantId: tenantID, AwsCognitoToken: token}

	var res ExchangeTokenResponse
	if err := AnonymousCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/ExchangeAWSCognitoJWT", req, DecodeJSONResponse(&res)); err != nil {
		return res, err

	}

	return res, nil
}

func IssueIdToken(ctx context.Context, aud string, version int) (IssueIdTokenResponse, error) {
	req := IssueIdTokenRequest{
		Audience: aud,
		Version:  version,
	}

	var res IssueIdTokenResponse
	if err := AuthenticatedCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/IssueIdToken", req, DecodeJSONResponse(&res)); err != nil {
		return IssueIdTokenResponse{}, err
	}

	return res, nil
}

func IssueIngressAccessToken(ctx context.Context, instanceId string) (IssueIngressAccessTokenResponse, error) {
	req := IssueIngressAccessTokenRequest{
		InstanceId: instanceId,
	}

	var res IssueIngressAccessTokenResponse
	if err := AuthenticatedCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/IssueIngressAccessToken", req, DecodeJSONResponse(&res)); err != nil {
		return IssueIngressAccessTokenResponse{}, err
	}

	return res, nil
}

func IssueDevelopmentToken(ctx context.Context) (string, error) {
	tok, err := FetchToken(ctx)
	if err != nil {
		return "", err
	}

	if tok.IsSessionToken() {
		return tok.IssueToken(ctx, time.Hour, IssueTenantTokenFromSession, true)
	}

	req := struct{}{}

	var res IssueDevelopmentTokenResponse
	if err := AuthenticatedCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/IssueDevelopmentToken", req, DecodeJSONResponse(&res)); err != nil {
		return "", err
	}

	return res.DevelopmentToken, nil
}

func TrustAWSCognitoJWT(ctx context.Context, tenantID, identityPool, identityProvider string) error {
	req := TrustAWSCognitoIdentityPoolRequest{AwsCognitoIdentityPool: identityPool, IdentityProvider: identityProvider}

	token, err := FetchToken(ctx)
	if err != nil {
		return err
	}

	claims, err := token.Claims(ctx)
	if err != nil {
		return err
	}

	if claims.TenantID != tenantID {
		return fnerrors.New("authenticated as %q, wanted %q", claims.TenantID, tenantID)
	}

	return Call[any]{
		Method: "nsl.tenants.TenantsService/TrustAWSCognitoIdentityPool",
		IssueBearerToken: func(ctx context.Context) (ResolvedToken, error) {
			return IssueBearerTokenFromToken(ctx, token)
		},
	}.Do(ctx, req, StaticEndpoint(EndpointAddress), nil)
}

type GetTenantResponse struct {
	Tenant *Tenant `json:"tenant,omitempty"`
}

func GetTenant(ctx context.Context) (GetTenantResponse, error) {
	req := struct{}{}

	var res GetTenantResponse
	if err := AuthenticatedCall(ctx, EndpointAddress, "nsl.tenants.TenantsService/GetTenant", req, DecodeJSONResponse(&res)); err != nil {
		return GetTenantResponse{}, err
	}

	return res, nil
}
