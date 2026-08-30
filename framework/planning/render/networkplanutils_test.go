// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package render

import (
	"fmt"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"gotest.tools/assert"
	jsontesting "namespacelabs.dev/foundation/internal/planning/deploy/testing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/storage"
)

func TestNetworkPlanToSummary(t *testing.T) {
	assertSummary(t, "empty", &storage.NetworkPlan{})

	port := &schema.Endpoint_Port{ContainerPort: 1}
	assertSummary(t, "basic", &storage.NetworkPlan{
		LocalHostname:         "localhost",
		FocusedServerPackages: []string{"main_server1, main_server2"},
		Endpoints: []*storage.Endpoint{
			// HTTP endpoint
			{
				ServiceName:   "http",
				EndpointOwner: "my/http_service",
				Ports: []*schema.Endpoint_PortMap{{
					Port:         &schema.Endpoint_Port{ContainerPort: 123},
					ExportedPort: 123,
				}},
				ServerOwner: "main_http_server",
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{
					{Protocol: "http"},
				},
			},
			// GRPC endpoint
			{
				ServiceName:   "grpc_service",
				EndpointOwner: "my/grpc_service",
				Ports: []*schema.Endpoint_PortMap{{
					Port:         &schema.Endpoint_Port{ContainerPort: 234},
					ExportedPort: 234,
				}},
				ServerOwner: "main_grpc_server",
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{
					{Kind: "my.service.MyGrpcService", Protocol: "grpc"},
				},
			},
			// Public gRPC with local port
			{
				LocalPort: 12300,
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{
					{Kind: "my.service3.MyService", Protocol: "grpc"},
				},
			},
			// Public http with local port
			{
				LocalPort:       12301,
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{{Protocol: "http"}},
			},
			// Private http
			{
				Ports: []*schema.Endpoint_PortMap{{
					Port: &schema.Endpoint_Port{ContainerPort: 1234},
				}},
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{{Protocol: "http"}},
			},
			// Private grpc
			{
				Ports: []*schema.Endpoint_PortMap{{
					Port: &schema.Endpoint_Port{ContainerPort: 1235},
				}},
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{
					{Kind: "my.service4.MyPrivateService", Protocol: "grpc"},
				},
			},
			// Various corner-cases
			{
				ServiceName: "internal",
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{
					{Kind: "internal-service"},
				},
				Ports: []*schema.Endpoint_PortMap{{Port: port}},
			},
			{
				ServiceName: "no-port",
			},
			{
				Ports:       []*schema.Endpoint_PortMap{{Port: port}},
				ServiceName: "http",
				ServerOwner: "my-http-server",
			},
			{
				Ports:       []*schema.Endpoint_PortMap{{Port: port}},
				ServerName:  "my-server1",
				ServiceName: "grpc-gateway",
			},
			{
				Ports:        []*schema.Endpoint_PortMap{{Port: port}},
				ServerName:   "my-server2",
				ServiceLabel: "service-label",
			},
			{
				Ports:       []*schema.Endpoint_PortMap{{Port: port}},
				ServerName:  "my-server3",
				ServiceName: "my-service3",
			},
			{
				Ports:        []*schema.Endpoint_PortMap{{Port: port}},
				ServiceLabel: "my-service-label1",
			},
			{
				Ports:       []*schema.Endpoint_PortMap{{Port: port}},
				ServiceName: "my-service-name1",
			},
			{
				Ports:       []*schema.Endpoint_PortMap{{Port: port}},
				LocalPort:   1236,
				ServiceName: "with-local-port",
			},
			// Verifying sorting by port
			{ServiceName: "ingress", LocalPort: 567},
			{ServiceName: "ingress", LocalPort: 456},
		},
		IngressFragments: []*storage.IngressFragment{
			{
				Owner: "my/http_service",
				Domain: &storage.Domain{
					Fqdn:        "domain1.example.com",
					Managed:     storage.Domain_USER_SPECIFIED,
					TlsFrontend: true,
				},
				Endpoint: &storage.Endpoint{
					ServiceName: "service1",
				},
				HttpPath: []*storage.IngressHttpPath{
					{Path: "/path1", Owner: "owner1", ServicePort: 123},
					{Path: "/path2", Owner: "owner2", ServicePort: 123},
				},
			},
			{
				Owner: "my/grpc_service",
				Domain: &storage.Domain{
					Fqdn:        "local.domain",
					Managed:     storage.Domain_LOCAL_MANAGED,
					TlsFrontend: false,
				},
				Endpoint: &storage.Endpoint{
					ServiceName:   "grpc_service",
					EndpointOwner: "my/grpc_service",
				},
				HttpPath: []*storage.IngressHttpPath{
					{Path: "/grpc-transcoding2/", Owner: "my/grpc_service", ServicePort: 123},
				},
				GrpcService: []*storage.IngressGrpcService{
					{GrpcService: "my.service.MyGrpcService"}},
			},
		},
	})
}

func assertSummary(t *testing.T, snapshotName string, plan *storage.NetworkPlan) {
	summary := NetworkPlanToSummary(plan)

	json := jsontesting.StableProtoToJson(t, summary)

	assert.NilError(t, cupaloy.SnapshotMulti(fmt.Sprintf("%s.json", snapshotName), json))
}
