// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package runtime

import (
	"testing"

	"gotest.tools/assert"
	"namespacelabs.dev/foundation/schema"
)

func TestIngressPort(t *testing.T) {
	serverPort := &schema.Endpoint_PortMap{ExportedPort: 4000, Port: &schema.Endpoint_Port{Name: "server-port", ContainerPort: 4000}}
	httpPort := &schema.Endpoint_PortMap{ExportedPort: 4001, Port: &schema.Endpoint_Port{Name: "http-port", ContainerPort: 4001}}
	grpcPort := &schema.Endpoint_PortMap{ExportedPort: 4002, Port: &schema.Endpoint_Port{Name: "grpc-port", ContainerPort: 4002}}
	endpoint := &schema.Endpoint{Ports: []*schema.Endpoint_PortMap{serverPort, httpPort, grpcPort}}

	assert.Equal(t, PortForProtocol(endpoint, schema.ClearTextGrpcProtocol), grpcPort)
	assert.Equal(t, PortForProtocol(endpoint, schema.HttpProtocol), httpPort)
	assert.Equal(t, PortForProtocol(endpoint, schema.GrpcProtocol), serverPort)
	assert.Equal(t, PortForProtocol(endpoint, schema.HttpsProtocol), serverPort)
	assert.Equal(t, PortForProtocol(&schema.Endpoint{Ports: []*schema.Endpoint_PortMap{httpPort, serverPort}}, schema.GrpcProtocol), serverPort)
	assert.Equal(t, PortForProtocol(&schema.Endpoint{Ports: []*schema.Endpoint_PortMap{serverPort}}, schema.ClearTextGrpcProtocol), serverPort)
	assert.Equal(t, PortForProtocol(endpoint, "custom"), serverPort)
}
