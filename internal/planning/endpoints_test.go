// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import (
	"testing"

	"gotest.tools/assert"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
)

type endpointTestPlanner struct {
	runtime.Planner
}

func (endpointTestPlanner) MakeServiceName(name string) (string, string) {
	return name, name + ".example.com"
}

func TestComputeEndpointsUsesInternalGrpcPortForDefaultGrpcListener(t *testing.T) {
	srv := endpointTestServer()
	ports := []*schema.Endpoint_Port{
		{Name: "server-port", ContainerPort: 4000},
		{Name: "grpc-port", ContainerPort: 4001},
		{Name: "int-grpc-port", ContainerPort: 4002},
		{Name: "int-http-port", ContainerPort: 4003},
	}

	endpoints, _, err := ComputeEndpoints(endpointTestPlanner{}, srv, &schema.ServerFragment{}, ports)
	assert.NilError(t, err)
	assert.Equal(t, len(endpoints), 1)
	assert.Equal(t, len(endpoints[0].Ports), 3)
	assert.Equal(t, endpoints[0].Ports[0].Port.Name, "server-port")
	assert.Equal(t, endpoints[0].Ports[1].Port.Name, "int-grpc-port")
	assert.Equal(t, endpoints[0].Ports[2].Port.Name, "int-http-port")
	assert.Equal(t, endpoints[0].ServiceMetadata[0].Protocol, schema.ClearTextGrpcProtocol)
}

func TestComputeEndpointsFallsBackToServerPort(t *testing.T) {
	endpoints, _, err := ComputeEndpoints(endpointTestPlanner{}, endpointTestServer(), &schema.ServerFragment{}, []*schema.Endpoint_Port{{Name: "server-port", ContainerPort: 4000}})
	assert.NilError(t, err)
	assert.Equal(t, len(endpoints), 1)
	assert.Equal(t, len(endpoints[0].Ports), 1)
	assert.Equal(t, endpoints[0].Ports[0].Port.Name, "server-port")
}

func TestComputeEndpointsPreservesExplicitExportedPort(t *testing.T) {
	srv := endpointTestServer()
	srv.entry.Node[0].ExportedPort = 8443

	endpoints, _, err := ComputeEndpoints(endpointTestPlanner{}, srv, &schema.ServerFragment{}, []*schema.Endpoint_Port{
		{Name: "server-port", ContainerPort: 4000},
		{Name: "grpc-port", ContainerPort: 4001},
	})
	assert.NilError(t, err)
	assert.Equal(t, endpoints[0].Ports[0].ExportedPort, int32(8443))
	assert.Equal(t, endpoints[0].Ports[0].Port.Name, "server-port")
	assert.Equal(t, endpoints[0].Ports[1].ExportedPort, int32(4001))
	assert.Equal(t, endpoints[0].Ports[1].Port.Name, "grpc-port")
}

func TestComputeEndpointsAvoidsPlaintextGrpcPortCollision(t *testing.T) {
	srv := endpointTestServer()
	srv.entry.Node[0].ExportedPort = 4001

	endpoints, _, err := ComputeEndpoints(endpointTestPlanner{}, srv, &schema.ServerFragment{}, []*schema.Endpoint_Port{
		{Name: "server-port", ContainerPort: 4000},
		{Name: "grpc-port", ContainerPort: 4001},
	})
	assert.NilError(t, err)
	assert.Equal(t, endpoints[0].Ports[0].ExportedPort, int32(4001))
	assert.Equal(t, endpoints[0].Ports[1].ExportedPort, int32(4000))
}

func TestComputeEndpointsExposesPlaintextGrpcPortForAllEndpointTypes(t *testing.T) {
	for _, endpointType := range []schema.Endpoint_Type{schema.Endpoint_PRIVATE, schema.Endpoint_LOAD_BALANCER} {
		t.Run(endpointType.String(), func(t *testing.T) {
			srv := endpointTestServer()
			srv.entry.Node[0].Ingress = endpointType

			endpoints, _, err := ComputeEndpoints(endpointTestPlanner{}, srv, &schema.ServerFragment{}, []*schema.Endpoint_Port{
				{Name: "server-port", ContainerPort: 4000},
				{Name: "grpc-port", ContainerPort: 4001},
			})
			assert.NilError(t, err)
			assert.Equal(t, len(endpoints[0].Ports), 2)
			assert.Equal(t, endpoints[0].Ports[0].Port.Name, "server-port")
			assert.Equal(t, endpoints[0].Ports[1].Port.Name, "grpc-port")
		})
	}
}

func endpointTestServer() Server {
	return Server{
		entry: &schema.Stack_Entry{
			Server: &schema.Server{Id: "server", PackageName: "example.com/server"},
			Node: []*schema.Node{{
				Kind:               schema.Node_SERVICE,
				PackageName:        "example.com/service",
				IngressServiceName: "service",
				ExportService:      []*schema.GrpcExportService{{ProtoTypename: "example.Service"}},
				Ingress:            schema.Endpoint_INTERNET_FACING,
			}},
		},
	}
}
