// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"namespacelabs.dev/foundation/framework/testing"
	"namespacelabs.dev/foundation/internal/testdata/service/proto"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/std/networking/gateway/server", "grpc-http-transcoder")

		if err := t.WaitForEndpoint(ctx, endpoint); err != nil {
			return err
		}

		// Lets check if the grpc service is being transcoded to http.

		// internal.testdata.service.proto.PostService is hosted in server/gogrpc.
		if err := makeTest(endpoint.Address(), match[*proto.PostResponse]{
			ServiceName: "internal.testdata.service.proto.PostService",
			MethodName:  "Post",
			Request:     &proto.PostRequest{Input: "xyz"},
			Response:    &proto.PostResponse{},
			Match: func(pr *proto.PostResponse) error {
				if pr.Id == "" {
					return errors.New("expected id to be set")
				}

				if pr.Response != "hello there: xyz" {
					return fmt.Errorf("unexpected response %q", pr.Response)
				}

				return nil
			},
		}); err != nil {
			log.Fatal(err)
		}

		return nil
	})
}

type match[V any] struct {
	ServiceName string
	MethodName  string
	Request     interface{}
	Response    V
	Match       func(V) error
}

func makeTest[V any](address string, m match[V]) error {
	serialized, err := json.Marshal(m.Request)
	if err != nil {
		return err
	}

	response, err := http.Post(fmt.Sprintf("http://%s/%s/%s", address, m.ServiceName, m.MethodName), "application/json", bytes.NewReader(serialized))
	if err != nil {
		return err
	}

	if response.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %s", response.Status)
	}

	x, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(x, m.Response); err != nil {
		return err
	}

	return m.Match(m.Response)
}
