// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import * as grpc from "@grpc/grpc-js";

export interface GrpcRegistrar {
	registerGrpcService(
		service: grpc.ServiceDefinition,
		implementation: grpc.UntypedServiceImplementation
	): void;
}
