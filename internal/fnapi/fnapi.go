// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/pflag"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
)

var (
	EndpointAddress = "https://api.namespacelabs.net"
	AdminMode       = false
)

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&EndpointAddress, "fnapi_endpoint", EndpointAddress, "The fnapi endpoint address.")
	_ = flags.MarkHidden("fnapi_endpoint")
	flags.BoolVar(&AdminMode, "fnapi_admin", AdminMode, "Whether to enable admin mode.")
	_ = flags.MarkHidden("fnapi_admin")
}

// A nil handle indicates that the caller wants to discard the response.
func AnonymousCall(ctx context.Context, endpoint string, method string, req interface{}, handle func(io.Reader) error) error {
	return Call[any]{
		Endpoint:  endpoint,
		Method:    method,
		Anonymous: true, // Callers of this API do not assume that credentials are injected.
	}.Do(ctx, req, handle)
}

func AuthenticatedCall(ctx context.Context, endpoint string, method string, req interface{}, handle func(io.Reader) error) error {
	return Call[any]{
		Endpoint: endpoint,
		Method:   method,
	}.Do(ctx, req, handle)
}

type Call[RequestT any] struct {
	Endpoint               string
	Method                 string
	PreAuthenticateRequest func(*auth.UserAuth, *RequestT) error
	Anonymous              bool
	OptionalAuth           bool // Don't fail if not authenticated.
}

func DecodeJSONResponse(resp any) func(io.Reader) error {
	return func(body io.Reader) error {
		return json.NewDecoder(body).Decode(resp)
	}
}

func AddNamespaceHeaders(ctx context.Context, headers *http.Header) {
	if tel := TelemetryOn(ctx); tel != nil && tel.IsTelemetryEnabled() {
		headers.Add("NS-Client-ID", tel.GetClientID())
	}

	headers.Add("NS-Internal-Version", fmt.Sprintf("%d", versions.Builtin().APIVersion))

	if AdminMode {
		headers.Add("NS-API-Mode", "admin")
	}
}

func getUserToken(ctx context.Context) (*auth.UserAuth, string, error) {
	var authOpts []auth.AuthOpt

	user, err := auth.LoadUser()
	switch {
	case err == nil:
		authOpts = append(authOpts, auth.WithUserAuth(user))
	case errors.Is(err, auth.ErrRelogin) && os.Getenv("GITHUB_ACTIONS") == "true":
		authOpts = append(authOpts, auth.WithGithubOIDC(true))
	default:
		return nil, "", err
	}

	tok, err := auth.GenerateToken(ctx, authOpts...)
	if err != nil {
		return nil, "", err
	}

	return user, tok, nil
}

func (c Call[RequestT]) Do(ctx context.Context, request RequestT, handle func(io.Reader) error) error {
	headers := http.Header{}

	if !c.Anonymous {
		user, tok, err := getUserToken(ctx)
		if err != nil && !c.OptionalAuth {
			return err
		}
		headers.Add("Authorization", "Bearer "+tok)

		if c.PreAuthenticateRequest != nil && user != nil {
			if err := c.PreAuthenticateRequest(user, &request); err != nil {
				return err
			}
		}
	}

	AddNamespaceHeaders(ctx, &headers)

	reqBytes, err := json.Marshal(request)
	if err != nil {
		return fnerrors.InternalError("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/"+c.Method, bytes.NewReader(reqBytes))
	if err != nil {
		return fnerrors.InternalError("failed to construct request: %w", err)
	}

	for k, v := range headers {
		httpReq.Header[k] = append(httpReq.Header[k], v...)
	}

	client := &http.Client{
		Transport: http.DefaultTransport,
	}

	response, err := client.Do(httpReq)
	if err != nil {
		return fnerrors.InvocationError("namespace api", "http call failed: %w", err)
	}

	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		if handle == nil {
			return nil
		}

		return handle(response.Body)
	}

	st := &spb.Status{}
	dec := json.NewDecoder(response.Body)
	if err := dec.Decode(st); err == nil {
		if st.Code == int32(codes.Unauthenticated) {
			return auth.ErrRelogin
		}

		return fnerrors.InvocationError("namespace api", "failed to call %s/%s: %w", c.Endpoint, c.Method, status.ErrorProto(st))
	}

	grpcMessage := response.Header[http.CanonicalHeaderKey("grpc-message")]
	grpcStatus := response.Header[http.CanonicalHeaderKey("grpc-status")]

	if len(grpcMessage) > 0 && len(grpcStatus) > 0 {
		intVar, err := strconv.Atoi(grpcStatus[0])
		if err == nil {
			st.Code = int32(intVar)
			st.Message = grpcMessage[0]

			switch st.Code {
			case int32(codes.PermissionDenied):
				return fnerrors.NoAccessToLimitedFeature()
			}

			return fnerrors.InvocationError("namespace api", "failed to call %s/%s: %w", c.Endpoint, c.Method, status.ErrorProto(st))
		}
	}

	switch response.StatusCode {
	case http.StatusInternalServerError:
		return fnerrors.InvocationError("namespace api", "internal server error, and wasn't able to parse error response")
	case http.StatusForbidden:
		return fnerrors.NoAccessToLimitedFeature()
	case http.StatusUnauthorized:
		return auth.ErrRelogin
	default:
		return fnerrors.InvocationError("namespace api", "unexpected %d error reaching %q: %s", response.StatusCode, c.Endpoint, response.Status)
	}
}
