// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import * as grpc from "@grpc/grpc-js";

export interface CallOptions {}

// Converts a Google gRPC client to Foundation's gRPC client.
// Called by the codegen.
export function adaptClient(
	clientCtr: grpc.ServiceClientConstructor,
	address: string,
	credentials: grpc.ChannelCredentials,
	options?: object
): any {
	const wrappedClient = new clientCtr(address, credentials, options);
	const result: { [methodName: string]: any } = {};
	Object.keys(clientCtr.service).forEach((methodName) => {
		const handleCall = wrappedClient[methodName].bind(wrappedClient);
		result[methodName] = async (request: any) => {
			return new Promise((resolve, reject) => {
				handleCall(request, (err: any, response: any) => {
					if (err != null) {
						reject(err);
					} else {
						resolve(response);
					}
				});
			});
		};
	});
	return result;
}
