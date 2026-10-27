// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package k8sdetector

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"namespacelabs.dev/foundation/std/core/types"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

func TestCreateResourceWithKubernetesDetector(t *testing.T) {
	t.Setenv("TRACING_K8S_NAMESPACE", "test-namespace")
	t.Setenv("TRACING_K8S_POD_NAME", "test-pod")
	t.Setenv("TRACING_K8S_POD_UID", "test-uid")
	t.Setenv("TRACING_K8S_NODE_NAME", "test-node")

	r, err := tracing.CreateResource(t.Context(), &types.ServerInfo{ServerName: "test"}, []resource.Detector{detector{}})
	if !assert.NoError(t, err) {
		return
	}

	assert.Contains(t, r.Attributes(), attribute.String("k8s.namespace.name", "test-namespace"))
	assert.Contains(t, r.Attributes(), attribute.String("k8s.pod.name", "test-pod"))
	assert.Contains(t, r.Attributes(), attribute.String("k8s.pod.uid", "test-uid"))
	assert.Contains(t, r.Attributes(), attribute.String("k8s.node.name", "test-node"))
}
