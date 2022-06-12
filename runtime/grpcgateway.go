// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package runtime

import (
	"fmt"

	anypb "google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

const GrpcHttpTranscodeNode schema.PackageName = "namespacelabs.dev/foundation/std/grpc/httptranscoding"

var (
	UseGoInternalGrpcGateway = false
)

const (
	GrpcGatewayServiceName = "grpc-gateway"

	// XXX this is not quite right; it's just a simple mechanism for language and runtime
	// to communicate. Ideally the schema would incorporate a gRPC map.
	kindNeedsGrpcGateway   = "needs-grpc-gateway"
	grpcGatewayServiceKind = "grpc-gateway"
)

func makeGrpcGatewayEndpoint(server *schema.Server, serverPorts []*schema.Endpoint_Port, gatewayServices []string, publicGateway bool) (*schema.Endpoint, error) {
	var gwPort *schema.Endpoint_Port
	for _, port := range serverPorts {
		if port.Name == "grpc-gateway-port" {
			gwPort = port
			break
		}
	}

	// This entrypoint is otherwise open to any caller, so follow the same
	// policy for browser-based requests.
	cors := &schema.HttpCors{Enabled: true, AllowedOrigin: []string{"*"}}
	packedCors, err := anypb.New(cors)
	if err != nil {
		return nil, fnerrors.UserError(nil, "failed to pack CORS' configuration: %v", err)
	}

	urlMap := &schema.HttpUrlMap{}
	for _, svc := range gatewayServices {
		urlMap.Entry = append(urlMap.Entry, &schema.HttpUrlMap_Entry{
			PathPrefix: fmt.Sprintf("/%s/", svc),
			Kind:       grpcGatewayServiceKind,
		})
	}
	packedUrlMap, err := anypb.New(urlMap)
	if err != nil {
		return nil, fnerrors.InternalError("failed to marshal url map: %w", err)
	}

	gwEndpoint := &schema.Endpoint{
		Type:          schema.Endpoint_PRIVATE,
		ServiceName:   GrpcGatewayServiceName,
		Port:          gwPort,
		AllocatedName: grpcGatewayName(server),
		EndpointOwner: server.GetPackageName(),
		ServerOwner:   server.GetPackageName(),
		ServiceMetadata: []*schema.ServiceMetadata{
			{Protocol: "http", Details: packedUrlMap},
			{Protocol: "http", Kind: "http-extension", Details: packedCors},
		},
	}

	if publicGateway {
		gwEndpoint.Type = schema.Endpoint_INTERNET_FACING
	}

	return gwEndpoint, nil
}

func grpcGatewayName(srv *schema.Server) string {
	return GrpcGatewayServiceName + "-" + srv.Id
}
