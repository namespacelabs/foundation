// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import * as grpc from "@grpc/grpc-js";

export interface CallContext {}

// Instances of this interface should only be created by "define*" methods in the generated files.
export interface BoundService<T> {
	impl: T;
	definition: grpc.ServiceDefinition;
}

// Converts a Foundation's bound gRPC service server to a Google service definition,
// suitable as "addService" arguments.
export function adaptServer<T>(
	serviceDef: BoundService<T>
): [grpc.ServiceDefinition, grpc.UntypedServiceImplementation] {
	const syncImpl: grpc.UntypedServiceImplementation = {};
	Object.keys(serviceDef.definition).forEach((methodName) => {
		const handleCall = (serviceDef.impl as any)[methodName].bind(serviceDef.impl);

		syncImpl[methodName] = async (
			call: grpc.ServerUnaryCall<any, any>,
			callback: grpc.sendUnaryData<any>
		) => {
			const context: CallContext = {};
			callback(null, await handleCall(call.request, context));
		};
	});

	return [serviceDef.definition, syncImpl];
}
