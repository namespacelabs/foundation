// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package filter

import (
	"os"
	"strings"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const DefaultEnvKey = "FOUNDATION_TRACE_SKIP_SPANS"

func FilterFromEnv(key string) sdktrace.Sampler {
	if v := os.Getenv(key); v != "" {
		m := map[string]struct{}{}
		for _, name := range strings.Split(v, ",") {
			m[strings.TrimSpace(name)] = struct{}{}
		}

		return sampler{m}
	}

	return nil
}

type sampler struct {
	drop map[string]struct{}
}

func (s sampler) ShouldSample(params sdktrace.SamplingParameters) sdktrace.SamplingResult {
	parent := trace.SpanContextFromContext(params.ParentContext)

	// If the parent is valid and sampled, inherit
	if parent.IsValid() && parent.IsSampled() {
		return sdktrace.SamplingResult{Decision: sdktrace.RecordAndSample}
	}

	if _, shouldDrop := s.drop[params.Name]; shouldDrop {
		return sdktrace.SamplingResult{Decision: sdktrace.Drop}
	}

	return sdktrace.SamplingResult{Decision: sdktrace.RecordAndSample}
}

func (sampler) Description() string { return "Foundation filter: reject by name" }
