// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/go-ids"
	"namespacelabs.dev/integrations/nsc/apienv"
)

var (
	globalEndpointOverride string
	AdminMode              = false
	DebugApiResponse       = false

	UserAgent = "ns/unknown"
)

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&globalEndpointOverride, "fnapi_endpoint", "", "The fnapi endpoint address.")
	_ = flags.MarkHidden("fnapi_endpoint")
	flags.BoolVar(&AdminMode, "fnapi_admin", AdminMode, "Whether to enable admin mode.")
	_ = flags.MarkHidden("fnapi_admin")
}

func GlobalEndpoint() string {
	if globalEndpointOverride != "" {
		return globalEndpointOverride
	}

	return apienv.GlobalEndpoint()
}

func ResolveGlobalEndpoint(ctx context.Context, tok ResolvedToken) (string, error) {
	return GlobalEndpoint(), nil
}

func ResolveIAMEndpoint(ctx context.Context, tok ResolvedToken) (string, error) {
	if globalEndpointOverride != "" {
		return globalEndpointOverride, nil
	}

	return apienv.IAMEndpoint(), nil
}

// A nil handle indicates that the caller wants to discard the response.
func AnonymousCall(ctx context.Context, endpoint ResolveFunc, method string, req interface{}, handle func(io.Reader) error) error {
	return Call[any]{
		Method:           method,
		IssueBearerToken: nil, // Callers of this API do not assume that credentials are injected.
	}.Do(ctx, req, endpoint, handle)
}

func AuthenticatedCall(ctx context.Context, endpoint ResolveFunc, method string, req interface{}, handle func(io.Reader) error) error {
	return Call[any]{
		Method:           method,
		IssueBearerToken: IssueBearerToken,
	}.Do(ctx, req, endpoint, handle)
}

func IssueTenantTokenFromSession(ctx context.Context, t *auth.Token, duration time.Duration) (string, error) {
	if t == nil {
		return "", fnerrors.New("can't issue tenant token")
	}

	if t.SessionToken == "" {
		// Retain old functionality.
		return t.TenantToken, nil
	}

	req := IssueTenantTokenFromSessionRequest{
		TokenDurationSecs: int64(duration.Seconds()),
	}

	var resp IssueTenantTokenFromSessionResponse

	if err := (Call[IssueTenantTokenFromSessionRequest]{
		Method: "nsl.signin.SigninService/IssueTenantTokenFromSession",
		IssueBearerToken: func(ctx context.Context) (ResolvedToken, error) {
			return ResolvedToken{BearerToken: t.SessionToken}, nil
		},
		ScrubRequest: func(req *IssueTenantTokenFromSessionRequest) {
			if req.SessionToken != "" {
				req.SessionToken = "scrubbed"
			}
		},
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&resp)); err != nil {
		return "", err
	}

	return resp.TenantToken, nil
}

func IssueSessionClientCertFromSession(ctx context.Context, sessionToken string, publicKeyPem string) (string, error) {
	req := IssueSessionClientCertFromSessionRequest{
		PublicKeyPem: publicKeyPem,
	}

	var resp IssueSessionClientCertFromSessionResponse

	if err := (Call[IssueSessionClientCertFromSessionRequest]{
		Method: "nsl.signin.SigninService/IssueSessionClientCertFromSession",
		IssueBearerToken: func(ctx context.Context) (ResolvedToken, error) {
			return ResolvedToken{BearerToken: sessionToken}, nil
		},
	}).Do(ctx, req, ResolveIAMEndpoint, DecodeJSONResponse(&resp)); err != nil {
		return "", err
	}

	return resp.ClientCertificatePem, nil
}

type Call[RequestT any] struct {
	Method           string
	IssueBearerToken func(context.Context) (ResolvedToken, error)
	ScrubRequest     func(*RequestT)
	Retryable        bool
}

func DecodeJSONResponse(resp any) func(io.Reader) error {
	return func(body io.Reader) error {
		return json.NewDecoder(body).Decode(resp)
	}
}

func AddNamespaceHeaders(headers http.Header) {
	headers.Add("NS-Internal-Version", fmt.Sprintf("%d", versions.Builtin().APIVersion))
	headers.Add("User-Agent", UserAgent)

	if AdminMode {
		headers.Add("NS-API-Mode", "admin")
	}
}

func AddJsonNamespaceHeaders(ctx context.Context, headers http.Header) {
	AddNamespaceHeaders(headers)
	headers.Add("Content-Type", "application/json")
}

type ResolveFunc func(context.Context, ResolvedToken) (string, error)

func (c Call[RequestT]) Do(ctx context.Context, request RequestT, resolveEndpoint ResolveFunc, handle func(io.Reader) error) error {
	headers := http.Header{}

	var resolvedToken ResolvedToken
	if c.IssueBearerToken != nil {
		tok, err := c.IssueBearerToken(ctx)
		if err != nil {
			return err
		}

		resolvedToken = tok

		headers.Add("Authorization", "Bearer "+tok.BearerToken)
	}

	AddJsonNamespaceHeaders(ctx, headers)

	reqBytes, err := json.Marshal(request)
	if err != nil {
		return fnerrors.InternalError("failed to marshal request: %w", err)
	}

	endpoint, err := resolveEndpoint(ctx, resolvedToken)
	if err != nil {
		return err
	}

	tid := ids.NewRandomBase32ID(4)
	fmt.Fprintf(console.Debug(ctx), "[%s] RPC: %v (endpoint: %v)\n", tid, c.Method, endpoint)

	reqDebugBytes := reqBytes
	if c.ScrubRequest != nil {
		c.ScrubRequest(&request)
		reqDebugBytes, _ = json.Marshal(request)
	}

	fmt.Fprintf(console.Debug(ctx), "[%s] Body: %s\n", tid, reqDebugBytes)

	return callSideEffectFree(ctx, c.Retryable, func(ctx context.Context) error {
		t := time.Now()
		url := endpoint + "/" + c.Method
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
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
			// https://cs.opensource.google/go/go/+/master:src/net/net.go;l=628;drc=869932d700cf161c19eec65d66b9fe55482698db
			if errors.Is(err, context.DeadlineExceeded) {
				return fnerrors.InvocationError("namespace api", "unable to contact %v: %w", endpoint, err)
			}

			return fnerrors.InvocationError("namespace api", "http call failed: %w", err)
		}

		defer response.Body.Close()

		fmt.Fprintf(console.Debug(ctx), "[%s] RPC: %v: status %s took %v\n", tid, c.Method, response.Status, time.Since(t))

		if response.StatusCode == http.StatusOK {
			if handle == nil {
				return nil
			}

			if DebugApiResponse {
				respBody, err := io.ReadAll(response.Body)
				if err != nil {
					return fnerrors.InvocationError("namespace api", "reading response body: %w", err)
				}

				fmt.Fprintf(console.Debug(ctx), "[%s] Response Body: %s\n", tid, respBody)

				return handle(bytes.NewReader(respBody))
			}

			return handle(response.Body)
		}

		respBody, err := io.ReadAll(response.Body)
		if err != nil {
			return fnerrors.InvocationError("namespace api", "reading response body: %w", err)
		}

		st := &spb.Status{}
		if err := json.Unmarshal(respBody, st); err == nil {
			return handleGrpcStatus(url, st)
		} else {
			// Retry status parsing with a more forgiving type.
			st := struct {
				*spb.Status
				// Code might be passed as a lower-case string.
				Code json.RawMessage `json:"code"`
				// Details might contain invalid Base64 values, just ignore it.
				Details json.RawMessage `json:"details"`
			}{}
			if json.Unmarshal(respBody, &st) == nil {
				var code codes.Code
				if json.Unmarshal(bytes.ToUpper(st.Code), &code) == nil {
					st.Status.Code = int32(code)
					return handleGrpcStatus(url, st.Status)
				}
			}

			fmt.Fprintf(console.Debug(ctx), "did not receive an RPC status: %v\n", err)
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

			return handleGrpcStatus(url, st)
		}

		grpcMessage := response.Header[http.CanonicalHeaderKey("grpc-message")]
		grpcStatus := response.Header[http.CanonicalHeaderKey("grpc-status")]

		if len(grpcMessage) > 0 && len(grpcStatus) > 0 {
			intVar, err := strconv.ParseInt(grpcStatus[0], 10, 32)
			if err == nil {
				st.Code = int32(intVar)
				st.Message = grpcMessage[0]

				return handleGrpcStatus(url, st)
			}
		}

		switch response.StatusCode {
		case http.StatusInternalServerError:
			return fnerrors.InternalError("namespace api: internal server error: %s", string(respBody))
		case http.StatusUnauthorized:
			return fnerrors.ReauthError("%s requires authentication", url)
		case http.StatusForbidden:
			return fnerrors.PermissionDeniedError("%s denied access", url)
		case http.StatusNotFound:
			return fnerrors.InternalError("%s not found: %s", url, string(respBody))
		default:
			return fnerrors.InvocationError("namespace api", "unexpected %d error reaching %q: %s", response.StatusCode, url, response.Status)
		}
	})
}

func callSideEffectFree(ctx context.Context, retryable bool, method func(context.Context) error) error {
	if !retryable {
		return method(ctx)
	}

	b := &backoff.ExponentialBackOff{
		InitialInterval:     500 * time.Millisecond,
		RandomizationFactor: 0.5,
		Multiplier:          1.5,
		MaxInterval:         5 * time.Second,
		MaxElapsedTime:      2 * time.Minute,
		Clock:               backoff.SystemClock,
	}

	b.Reset()

	span := trace.SpanFromContext(ctx)

	return backoff.Retry(func() error {
		if methodErr := method(ctx); methodErr != nil {
			// grpc's ConnectionError have a Temporary() signature. If we, for example, write to
			// a channel and that channel is gone, then grpc observes a ECONNRESET. And propagates
			// it as a temporary error. It doesn't know though whether it's safe to retry, so it
			// doesn't.
			if temp, ok := methodErr.(interface{ Temporary() bool }); ok && temp.Temporary() {
				span.RecordError(methodErr, trace.WithAttributes(attribute.Bool("grpc.temporary_error", true)))
				return methodErr
			}

			var netErr *net.OpError
			if errors.As(methodErr, &netErr) {
				if errno, ok := netErr.Err.(syscall.Errno); ok && errno == syscall.ECONNRESET {
					return methodErr // Retry
				}
			}

			return backoff.Permanent(methodErr)
		}

		return nil
	}, backoff.WithContext(b, ctx))
}

func handleGrpcStatus(url string, st *spb.Status) error {
	switch st.Code {
	case int32(codes.Unauthenticated):
		return fnerrors.ReauthError("%s requires authentication: %w", url, status.ErrorProto(st))

	case int32(codes.PermissionDenied):
		return fnerrors.PermissionDeniedError("%s denied access: %w", url, status.ErrorProto(st))

	case int32(codes.FailedPrecondition):
		// Failed precondition is not retryable so we should not suggest that it is transient (e.g. invocation error suggests this).
		return fnerrors.Newf("failed to call %s: %w", url, status.ErrorProto(st))

	case int32(codes.Internal):
		return fnerrors.InternalError("failed to call %s: %w", url, status.ErrorProto(st))

	default:
		return fnerrors.InvocationError("namespace api", "failed to call %s: %w", url, status.ErrorProto(st))
	}
}
