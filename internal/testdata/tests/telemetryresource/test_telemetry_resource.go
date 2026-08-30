// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/testing"
	"namespacelabs.dev/foundation/internal/testdata/service/telemetryinfo"
)

func main() {
	testing.Do(func(ctx context.Context, t testing.Test) error {
		endpoint := t.MustEndpoint("namespacelabs.dev/foundation/internal/testdata/service/telemetryinfo", "telemetryinfo")

		conn, err := t.NewClient(endpoint)
		if err != nil {
			return err
		}

		cli := telemetryinfo.NewTelemetryInfoServiceClient(conn)
		resp, err := cli.GetServiceName(ctx, &telemetryinfo.GetServiceNameRequest{})
		if err != nil {
			return err
		}

		expectedServiceName := "custom-otel-service-name"
		if resp.TelemetryServiceName != expectedServiceName {
			return fmt.Errorf("expected telemetry service name to be %q, got %q", expectedServiceName, resp.TelemetryServiceName)
		}

		if resp.ServerName == "" {
			return fmt.Errorf("expected server name to be non-empty")
		}

		return nil
	})
}
