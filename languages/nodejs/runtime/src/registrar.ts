// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import * as grpc from "@grpc/grpc-js";
import { FastifyInstance } from "fastify";
export interface Registrar {
	registerGrpcService(
		service: grpc.ServiceDefinition,
		implementation: grpc.UntypedServiceImplementation
	): void;

	// Fastify provides a convenient fluent API so we expose it as the whole.
	http(): FastifyInstance;
}
