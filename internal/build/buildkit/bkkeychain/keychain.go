// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package bkkeychain

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/moby/buildkit/session/auth"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
)

type Wrapper struct {
	Context     context.Context // Solve's parent context.
	ErrorLogger io.Writer
	Keychain    oci.Keychain
}

func (kw Wrapper) Register(server *grpc.Server) {
	auth.RegisterAuthServer(server, kw)
}

func (kw Wrapper) Credentials(ctx context.Context, req *auth.CredentialsRequest) (*auth.CredentialsResponse, error) {
	response, err := kw.credentials(ctx, req.Host)

	if err == nil {
		fmt.Fprintf(console.Debug(kw.Context), "[buildkit] AuthServer.Credentials %q --> %q\n", req.Host, response.Username)
	} else {
		fmt.Fprintf(console.Debug(kw.Context), "[buildkit] AuthServer.Credentials %q: failed: %v\n", req.Host, err)

	}

	return response, err
}

func (kw Wrapper) credentials(ctx context.Context, host string) (*auth.CredentialsResponse, error) {
	// The parent context, not the incoming context is used, as the parent
	// context has an ActionSink attached (etc) while the incoming context is
	// managed by buildkit.
	authn, err := kw.Keychain.Resolve(kw.Context, resourceWrapper{host})
	if err != nil {
		return nil, err
	}

	if authn == nil {
		return &auth.CredentialsResponse{}, nil
	}

	authz, err := authn.Authorization()
	if err != nil {
		return nil, err
	}

	if authz.IdentityToken != "" || authz.RegistryToken != "" {
		fmt.Fprintf(kw.ErrorLogger, "%s: authentication type mismatch, got token expected username/secret", host)
		return nil, rpcerrors.Errorf(codes.InvalidArgument, "expected username/secret got token")
	}

	return &auth.CredentialsResponse{Username: authz.Username, Secret: authz.Password}, nil
}

func (kw Wrapper) FetchToken(ctx context.Context, req *auth.FetchTokenRequest) (*auth.FetchTokenResponse, error) {
	fmt.Fprintf(kw.ErrorLogger, "AuthServer.FetchToken %s\n", asJson(req))
	return nil, rpcerrors.Errorf(codes.Unimplemented, "unimplemented")
}

func (kw Wrapper) GetTokenAuthority(ctx context.Context, req *auth.GetTokenAuthorityRequest) (*auth.GetTokenAuthorityResponse, error) {
	fmt.Fprintf(kw.ErrorLogger, "AuthServer.GetTokenAuthority %s\n", asJson(req))
	return nil, rpcerrors.Errorf(codes.Unimplemented, "unimplemented")
}

func (kw Wrapper) VerifyTokenAuthority(ctx context.Context, req *auth.VerifyTokenAuthorityRequest) (*auth.VerifyTokenAuthorityResponse, error) {
	fmt.Fprintf(kw.ErrorLogger, "AuthServer.VerifyTokenAuthority %s\n", asJson(req))
	return nil, rpcerrors.Errorf(codes.Unimplemented, "unimplemented")
}

type resourceWrapper struct {
	host string
}

func (rw resourceWrapper) String() string      { return rw.host }
func (rw resourceWrapper) RegistryStr() string { return rw.host }

func asJson(msg any) string {
	str, _ := json.Marshal(msg)
	return string(str)
}
