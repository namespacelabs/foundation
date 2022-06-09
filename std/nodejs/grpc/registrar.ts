// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { BoundService } from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/server";

export interface GrpcRegistrar {
	registerGrpcService<T>(serviceDef: BoundService<T>): void;
}
