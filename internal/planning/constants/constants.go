// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package constants

const (
	HttpServiceName    = "http"
	IngressServiceName = "ingress"
	IngressServiceKind = "ingress"

	ManualInternalService = "internal-service"

	GrpcGatewayServiceName = "grpc-gateway"

	// XXX this is not quite right; it's just a simple mechanism for language and runtime
	// to communicate. Ideally the schema would incorporate a gRPC map.
	KindNeedsGrpcGateway = "needs-grpc-gateway"
)

var ReservedServiceNames = []string{HttpServiceName, GrpcGatewayServiceName, IngressServiceName}
