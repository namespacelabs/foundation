// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"time"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
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

type TokenClaims struct {
	TenantID      string `json:"tenant_id"`
	InstanceID    string `json:"instance_id"`
	OwnerID       string `json:"owner_id"`
	PrimaryRegion string `json:"primary_region"`
}

func Claims(tok Token) (*TokenClaims, error) {
	parts := strings.Split(tok.Raw(), ".")
	if len(parts) < 2 {
		return nil, fnerrors.New("invalid token")
	}

	dec, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fnerrors.New("invalid token: %w", err)
	}

	var claims TokenClaims
	if err := json.Unmarshal(dec, &claims); err != nil {
		return nil, fnerrors.New("invalid claims: %w", err)
	}

	return &claims, nil
}

func TrustAWSCognitoJWT(ctx context.Context, tenantID, identityPool, identityProvider string) error {
	req := TrustAWSCognitoIdentityPoolRequest{AwsCognitoIdentityPool: identityPool, IdentityProvider: identityProvider}

	token, err := FetchToken(ctx)
	if err != nil {
		return err
	}

	claims, err := Claims(token)
	if err != nil {
		return err
	}

	if claims.TenantID != tenantID {
		return fnerrors.New("authenticated as %q, wanted %q", claims.TenantID, tenantID)
	}

	return Call[any]{
		Method:     "nsl.tenants.TenantsService/TrustAWSCognitoIdentityPool",
		FetchToken: func(ctx context.Context) (Token, error) { return token, nil },
	}.Do(ctx, req, ResolveStaticEndpoint(EndpointAddress), nil)
}

type userToken string

func (u userToken) Raw() string {
	return string(u)
}

type ExchangeUserTokenRequest struct {
	Scopes []string `json:"scopes,omitempty"`
}

type ExchangeUserTokenResponse struct {
	TenantToken string `json:"tenant_token,omitempty"`
}

func ExchangeUserToken(ctx context.Context, token string, scopes ...string) (ExchangeUserTokenResponse, error) {
	req := ExchangeUserTokenRequest{Scopes: scopes}

	var res ExchangeUserTokenResponse
	if err := (Call[ExchangeUserTokenRequest]{
		Method: "nsl.tenants.TenantsService/ExchangeUserToken",
		FetchToken: func(ctx context.Context) (Token, error) {
			return userToken(token), nil
		},
	}.Do(ctx, req, ResolveStaticEndpoint(EndpointAddress), DecodeJSONResponse(&res))); err != nil {
		return ExchangeUserTokenResponse{}, err
	}

	return res, nil
}

func ResolveSpec() (string, error) {
	if spec := os.Getenv("NSC_TOKEN_SPEC"); spec != "" {
		return spec, nil
	}

	if specFile := os.Getenv("NSC_TOKEN_SPEC_FILE"); specFile != "" {
		contents, err := os.ReadFile(specFile)
		if err != nil {
			return "", fnerrors.New("failed to load spec: %w", err)
		}

		return string(contents), nil
	}

	return "", nil
}

func FetchToken(ctx context.Context) (Token, error) {
	return tasks.Return(ctx, tasks.Action("nsc.fetch-token").LogLevel(1), func(ctx context.Context) (*auth.Token, error) {
		spec, err := ResolveSpec()
		if err != nil {
			return nil, err
		}

		if spec != "" {
			return auth.FetchTokenFromSpec(ctx, spec)
		}

		if specified := os.Getenv("NSC_TOKEN_FILE"); specified != "" {
			return auth.LoadTokenFromPath(ctx, specified, time.Now())
		}

		return auth.LoadTenantToken(ctx)
	})
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
