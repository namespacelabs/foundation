// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/spf13/pflag"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
)

var (
	EndpointAddress             = "https://api.namespacelabs.net"
	AdminMode                   = false
	ExchangeGithubToTenantToken = false
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
		Endpoint:   endpoint,
		Method:     method,
		FetchToken: nil, // Callers of this API do not assume that credentials are injected.
	}.Do(ctx, req, handle)
}

func AuthenticatedCall(ctx context.Context, endpoint string, method string, req interface{}, handle func(io.Reader) error) error {
	return Call[any]{
		Endpoint: endpoint,
		Method:   method,
		FetchToken: func(ctx context.Context) (Token, error) {
			return FetchToken(ctx)
		},
	}.Do(ctx, req, handle)
}

type Token interface {
	Raw() string
}

func BearerToken(t Token) string {
	return "Bearer " + t.Raw()
}

type Call[RequestT any] struct {
	Endpoint   string
	Method     string
	FetchToken func(context.Context) (Token, error)
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

func (c Call[RequestT]) Do(ctx context.Context, request RequestT, handle func(io.Reader) error) error {
	headers := http.Header{}

	if c.FetchToken != nil {
		tok, err := c.FetchToken(ctx)
		if err != nil {
			return err
		}
		headers.Add("Authorization", BearerToken(tok))
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

	respBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fnerrors.InvocationError("namespace api", "reading response body: %w", err)
	}

	st := &spb.Status{}
	if err := json.Unmarshal(respBody, st); err == nil {
		return c.handleGrpcStatus(st)
	}

	fmt.Fprintf(console.Debug(ctx), "Error body response: %s\n", string(respBody))

	if grpcDetails := response.Header[http.CanonicalHeaderKey("grpc-status-details-bin")]; len(grpcDetails) > 0 {
		data, err := base64.RawStdEncoding.DecodeString(grpcDetails[0])
		if err != nil {
			return fnerrors.InternalError("failed to decode grpc details: %w", err)
		}

		if err := proto.Unmarshal(data, st); err != nil {
			return fnerrors.InternalError("failed to unmarshal grpc details: %w", err)
		}

		return c.handleGrpcStatus(st)
	}

	grpcMessage := response.Header[http.CanonicalHeaderKey("grpc-message")]
	grpcStatus := response.Header[http.CanonicalHeaderKey("grpc-status")]

	if len(grpcMessage) > 0 && len(grpcStatus) > 0 {
		intVar, err := strconv.Atoi(grpcStatus[0])
		if err == nil {
			st.Code = int32(intVar)
			st.Message = grpcMessage[0]

			return c.handleGrpcStatus(st)
		}
	}

	switch response.StatusCode {
	case http.StatusInternalServerError:
		return fnerrors.InternalError("namespace api: internal server error, and wasn't able to parse error response")
	case http.StatusUnauthorized:
		return fnerrors.ReauthError("%s/%s requires authentication: %w", c.Endpoint, c.Method, status.ErrorProto(st))
	default:
		return fnerrors.InvocationError("namespace api", "unexpected %d error reaching %q: %s", response.StatusCode, c.Endpoint, response.Status)
	}
}

func (c Call[RequestT]) handleGrpcStatus(st *spb.Status) error {
	switch st.Code {
	case int32(codes.Unauthenticated):
		return fnerrors.ReauthError("%s/%s requires authentication: %s", c.Endpoint, c.Method, st.Message)

	case int32(codes.FailedPrecondition):
		// Failed precondition is not retryable so we should not suggest that it is transient (e.g. invocation error suggests this).
		return fnerrors.New("failed to call %s/%s: %s", c.Endpoint, c.Method, st.Message)

	case int32(codes.Internal):
		return fnerrors.InternalError("failed to call %s/%s: %s", c.Endpoint, c.Method, st.Message)

	default:
		return fnerrors.InvocationError("namespace api", "failed to call %s/%s: %w", c.Endpoint, c.Method, status.ErrorProto(st))
	}
}
