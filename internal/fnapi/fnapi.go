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
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
)

var EndpointAddress = "https://api.namespacelabs.net"

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&EndpointAddress, "fnapi_endpoint", EndpointAddress, "The fnapi endpoint address.")
	_ = flags.MarkHidden("fnapi_endpoint")
}

// A nil handle indicates that the caller wants to discard the response.
func AnonymousCall(ctx context.Context, endpoint string, method string, req interface{}, handle func(io.Reader) error) error {
	return Call[any]{
		Endpoint:  endpoint,
		Method:    method,
		Anonymous: true, // Callers of this API do not assume that credentials are injected.
	}.Do(ctx, req, handle)
}

type Call[RequestT any] struct {
	Endpoint               string
	Method                 string
	PreAuthenticateRequest func(*UserAuth, *RequestT) error
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

	headers.Add("NS-Internal-Version", fmt.Sprintf("%d", versions.APIVersion))
}

func (c Call[RequestT]) Do(ctx context.Context, request RequestT, handle func(io.Reader) error) error {
	headers := http.Header{}

	if !c.Anonymous {
		user, err := LoadUser()
		if err != nil {
			if !c.OptionalAuth {
				return err
			}
		} else {
			headers.Add("Authorization", "Bearer "+base64.RawStdEncoding.EncodeToString(user.Opaque))

			if c.PreAuthenticateRequest != nil {
				if err := c.PreAuthenticateRequest(user, &request); err != nil {
					return err
				}
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
			return ErrRelogin
		}

		return status.ErrorProto(st)
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

			return status.ErrorProto(st)
		}
	}

	switch response.StatusCode {
	case http.StatusInternalServerError:
		return fnerrors.InvocationError("namespace api", "internal server error, and wasn't able to parse error response")
	case http.StatusForbidden:
		return fnerrors.NoAccessToLimitedFeature()
	case http.StatusUnauthorized:
		return ErrRelogin
	default:
		return fnerrors.InvocationError("namespace api", "unexpected %d error reaching %q: %s", response.StatusCode, c.Endpoint, response.Status)
	}
}
