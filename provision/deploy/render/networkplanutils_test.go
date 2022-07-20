// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package render

import (
	"fmt"
	"testing"

	"github.com/bradleyjkemp/cupaloy"
	"gotest.tools/assert"
	jsontesting "namespacelabs.dev/foundation/provision/deploy/testing"
	"namespacelabs.dev/foundation/schema/storage"
)

func TestNetworkPlanToSummary(t *testing.T) {
	assertSummary(t, "empty", &storage.NetworkPlan{})

	port := &storage.Endpoint_Port{ContainerPort: 1}
	assertSummary(t, "basic", &storage.NetworkPlan{
		LocalHostname:         "localhost",
		FocusedServerPackages: []string{"main_server1, main_server2"},
		Endpoints: []*storage.Endpoint{
			// HTTP endpoint
			{
				ServiceName:   "http",
				EndpointOwner: "my/http_service",
				Port:          &storage.Endpoint_Port{ContainerPort: 123},
				ServerOwner:   "main_http_server",
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{
					{Protocol: "http"},
				},
			},
			// GRPC endpoint
			{
				ServiceName:   "grpc_service",
				EndpointOwner: "my/grpc_service",
				Port:          &storage.Endpoint_Port{ContainerPort: 234},
				ServerOwner:   "main_grpc_server",
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
				Port:            &storage.Endpoint_Port{ContainerPort: 1234},
				ServiceMetadata: []*storage.Endpoint_ServiceMetadata{{Protocol: "http"}},
			},
			// Private grpc
			{
				Port: &storage.Endpoint_Port{ContainerPort: 1235},
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
				Port: port,
			},
			{
				ServiceName: "no-port",
			},
			{
				Port:        port,
				ServiceName: "http",
				ServerOwner: "my-http-server",
			},
			{
				Port:        port,
				ServerName:  "my-server1",
				ServiceName: "grpc-gateway",
			},
			{
				Port:         port,
				ServerName:   "my-server2",
				ServiceLabel: "service-label",
			},
			{
				Port:        port,
				ServerName:  "my-server3",
				ServiceName: "my-service3",
			},
			{
				Port:         port,
				ServiceLabel: "my-service-label1",
			},
			{
				Port:        port,
				ServiceName: "my-service-name1",
			},
			{
				Port:        port,
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
					Fqdn:           "domain1.example.com",
					Managed:        storage.Domain_USER_SPECIFIED,
					HasCertificate: true,
				},
				Endpoint: &storage.Endpoint{
					ServiceName: "service1",
				},
				HttpPath: []*storage.IngressHttpPath{
					{Path: "/path1", Owner: "owner1",
						Port: &storage.Endpoint_Port{ContainerPort: 123}},
					{Path: "/path2", Owner: "owner2",
						Port: &storage.Endpoint_Port{ContainerPort: 123}},
				},
			},
			{
				Owner: "my/grpc_service",
				Domain: &storage.Domain{
					Fqdn:           "local.domain",
					Managed:        storage.Domain_LOCAL_MANAGED,
					HasCertificate: false,
				},
				Endpoint: &storage.Endpoint{
					ServiceName:   "grpc_service",
					EndpointOwner: "my/grpc_service",
				},
				HttpPath: []*storage.IngressHttpPath{
					{Path: "/grpc-transcoding2/", Owner: "my/grpc_service",
						Port: &storage.Endpoint_Port{ContainerPort: 123}},
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
