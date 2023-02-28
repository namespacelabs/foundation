// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"namespacelabs.dev/foundation/framework/testing"
)

type targetType struct {
	Any       map[string]any `json:"any,omitempty"`
	Timestamp time.Time      `json:"timestamp,omitempty"`
	Duration  string         `json:"duration,omitempty"`
}

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/std/networking/gateway/server", "grpc-http-transcoder")

		url := testing.MakeHttpUrl(endpoint, "internal.testdata.service.proto.PostService/TestTranscoding")

		log.Printf("Request URL: %s\n", url)

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
		if err != nil {
			return fmt.Errorf("failed to construct request: %w", err)
		}

		client := &http.Client{
			Transport: http.DefaultTransport,
		}

		response, err := client.Do(httpReq)
		if err != nil {
			return fmt.Errorf("http call failed: %w", err)
		}

		defer response.Body.Close()

		if response.StatusCode != http.StatusOK {
			return fmt.Errorf("got non-OK HTTP status: %d", response.StatusCode)
		}

		respBody, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("failed to read body: %w", err)
		}

		log.Printf("Response body: %s\n", string(respBody))

		// We use a custom target type to enforce variants on the wire format
		// instead of relying on protojson for restoring the original type.
		var res targetType
		if err := json.Unmarshal(respBody, &res); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)

		}

		if _, err := time.ParseDuration(res.Duration); err != nil {
			return fmt.Errorf("failed to parse duration: %w", err)
		}

		if res.Timestamp.IsZero() {
			return fmt.Errorf("Expected non-zero timestamp")
		}

		if typeUrl := res.Any["@type"]; typeUrl == "" {
			return fmt.Errorf("Expected type url to be provided inline")
		}

		return nil
	})
}
