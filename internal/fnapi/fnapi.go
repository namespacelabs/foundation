// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fnapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/spf13/pflag"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/fnerrors"
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
}

func DecodeJSONResponse(resp any) func(io.Reader) error {
	return func(body io.Reader) error {
		return json.NewDecoder(body).Decode(resp)
	}
}

func (c Call[RequestT]) Do(ctx context.Context, request RequestT, handle func(io.Reader) error) error {
	headers := http.Header{}

	if !c.Anonymous {
		user, err := LoadUser()
		if err != nil {
			return err
		}

		headers.Add("Authorization", "Bearer "+base64.RawStdEncoding.EncodeToString(user.Opaque))

		if c.PreAuthenticateRequest != nil {
			c.PreAuthenticateRequest(user, &request)
		}
	}

	reqBytes, err := json.Marshal(request)
	if err != nil {
		return fnerrors.InvocationError("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/"+c.Method, bytes.NewReader(reqBytes))
	if err != nil {
		return fnerrors.InvocationError("failed to construct request: %w", err)
	}

	for k, v := range headers {
		httpReq.Header[k] = append(httpReq.Header[k], v...)
	}

	client := &http.Client{}
	response, err := client.Do(httpReq)
	if err != nil {
		return fnerrors.InvocationError("failed to perform invocation: %w", err)
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
			return status.ErrorProto(st)
		}
	}

	switch response.StatusCode {
	case http.StatusInternalServerError:
		return fnerrors.InvocationError("internal server error, and wasn't able to parse error response")
	case http.StatusForbidden:
		return fnerrors.InvocationError("forbidden")
	case http.StatusUnauthorized:
		return ErrRelogin
	default:
		return fnerrors.InvocationError("unexpected %d error reaching %q: %s", response.StatusCode, c.Endpoint, response.Status)
	}
}
