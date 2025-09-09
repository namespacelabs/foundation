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

type ExchangeTenantTokenForClientCertRequest struct {
	PublicKeyPem string `json:"public_key_pem,omitempty"`
}

type ExchangeTenantTokenForClientCertResponse struct {
	ClientCertificatePem string `json:"client_certificate_pem,omitempty"`
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
	Audience     string `json:"audience,omitempty"`
	Version      int    `json:"version,omitempty"`
	DurationSecs int64  `json:"duration_secs,omitempty"`
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

	if err := (Call[ExchangeGithubTokenRequest]{
		Method:    "nsl.tenants.TenantsService/ExchangeGithubToken",
		Retryable: true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
		return ExchangeGithubTokenResponse{}, err
	}

	return res, nil
}

func ExchangeCircleciToken(ctx context.Context, token string) (ExchangeCircleciTokenResponse, error) {
	req := ExchangeCircleciTokenRequest{CircleciToken: token}

	var res ExchangeCircleciTokenResponse
	if err := (Call[ExchangeCircleciTokenRequest]{
		Method:    "nsl.tenants.TenantsService/ExchangeCircleciToken",
		Retryable: true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
		return ExchangeCircleciTokenResponse{}, err
	}

	return res, nil
}

func ExchangeOIDCToken(ctx context.Context, tenantID, token string) (ExchangeTokenResponse, error) {
	req := ExchangeOIDCTokenRequest{TenantId: tenantID, OidcToken: token}

	var res ExchangeTokenResponse
	if err := (Call[ExchangeOIDCTokenRequest]{
		Method:    "nsl.tenants.TenantsService/ExchangeOIDCToken",
		Retryable: true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
		return ExchangeTokenResponse{}, err
	}

	return res, nil
}

func ExchangeAWSCognitoJWT(ctx context.Context, tenantID, token string) (ExchangeTokenResponse, error) {
	req := ExchangeAWSCognitoJWTRequest{TenantId: tenantID, AwsCognitoToken: token}

	var res ExchangeTokenResponse
	if err := (Call[ExchangeAWSCognitoJWTRequest]{
		Method:    "nsl.tenants.TenantsService/ExchangeAWSCognitoJWT",
		Retryable: true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
		return ExchangeTokenResponse{}, err
	}

	return res, nil
}

func IssueIdToken(ctx context.Context, aud string, version int, duration time.Duration) (IssueIdTokenResponse, error) {
	req := IssueIdTokenRequest{
		Audience:     aud,
		Version:      version,
		DurationSecs: int64(duration.Seconds()),
	}

	var res IssueIdTokenResponse
	if err := (Call[IssueIdTokenRequest]{
		Method:           "nsl.tenants.TenantsService/IssueIdToken",
		IssueBearerToken: IssueBearerToken,
		Retryable:        true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
		return IssueIdTokenResponse{}, err
	}

	return res, nil
}

func IssueIngressAccessToken(ctx context.Context, instanceId string) (IssueIngressAccessTokenResponse, error) {
	req := IssueIngressAccessTokenRequest{
		InstanceId: instanceId,
	}

	var res IssueIngressAccessTokenResponse
	if err := (Call[IssueIngressAccessTokenRequest]{
		Method:           "nsl.tenants.TenantsService/IssueIngressAccessToken",
		IssueBearerToken: IssueBearerToken,
		Retryable:        true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
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
		return tok.IssueToken(ctx, time.Hour, true)
	}

	req := struct{}{}

	var res IssueDevelopmentTokenResponse
	if err := (Call[any]{
		Method:           "nsl.tenants.TenantsService/IssueDevelopmentToken",
		IssueBearerToken: IssueBearerToken,
		Retryable:        true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
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
		return fnerrors.Newf("authenticated as %q, wanted %q", claims.TenantID, tenantID)
	}

	return Call[TrustAWSCognitoIdentityPoolRequest]{
		Method: "nsl.tenants.TenantsService/TrustAWSCognitoIdentityPool",
		IssueBearerToken: func(ctx context.Context) (ResolvedToken, error) {
			return IssueBearerTokenFromToken(ctx, token)
		},
	}.Do(ctx, req, ResolveIAMEndpoint, nil)
}

func IssueTenantClientCertFromToken(ctx context.Context, token Token, publicKey string) (string, error) {
	var res ExchangeTenantTokenForClientCertResponse
	req := ExchangeTenantTokenForClientCertRequest{
		PublicKeyPem: publicKey,
	}

	if err := (Call[ExchangeTenantTokenForClientCertRequest]{
		Method: "nsl.tenants.TenantsService/ExchangeTenantTokenForClientCert",
		IssueBearerToken: func(ctx context.Context) (ResolvedToken, error) {
			return IssueBearerTokenFromToken(ctx, token)
		},

		Retryable: true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
		return "", err
	}

	return res.ClientCertificatePem, nil
}

type GetTenantResponse struct {
	Tenant *Tenant `json:"tenant,omitempty"`
}

type StoredTrustRelationship struct {
	Id           string     `json:"id,omitempty"`
	CreatorJson  string     `json:"creator_json,omitempty"`
	CreatedAt    *time.Time `json:"created_at,omitempty"`
	Issuer       string     `json:"issuer,omitempty"`
	SubjectMatch string     `json:"subject_match,omitempty"`
}

type UpdateTrustRelationshipsRequest struct {
	Generation         string                    `json:"generation"`
	TrustRelationships []StoredTrustRelationship `json:"trust_relationships,omitempty"`
}

type ListTrustRelationshipsResponse struct {
	Generation         string                    `json:"generation"`
	TrustRelationships []StoredTrustRelationship `json:"trust_relationships,omitempty"`
}

func GetTenant(ctx context.Context) (GetTenantResponse, error) {
	req := struct{}{}

	var res GetTenantResponse
	if err := (Call[any]{
		Method:           "nsl.tenants.TenantsService/GetTenant",
		IssueBearerToken: IssueBearerToken,
		Retryable:        true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
		return GetTenantResponse{}, err
	}

	return res, nil
}

func UpdateTrustRelationships(ctx context.Context, generation string, trustRelationships []StoredTrustRelationship) error {
	req := UpdateTrustRelationshipsRequest{
		Generation:         generation,
		TrustRelationships: trustRelationships,
	}

	return Call[UpdateTrustRelationshipsRequest]{
		Method:           "nsl.tenants.TenantsService/UpdateTrustRelationships",
		IssueBearerToken: IssueBearerToken,
		Retryable:        true,
	}.Do(ctx, req, ResolveIAMEndpoint, nil)
}

func ListTrustRelationships(ctx context.Context) (ListTrustRelationshipsResponse, error) {
	req := struct{}{}

	var res ListTrustRelationshipsResponse
	if err := (Call[any]{
		Method:           "nsl.tenants.TenantsService/ListTrustRelationships",
		IssueBearerToken: IssueBearerToken,
		Retryable:        true,
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&res)); err != nil {
		return ListTrustRelationshipsResponse{}, err
	}

	return res, nil
}
