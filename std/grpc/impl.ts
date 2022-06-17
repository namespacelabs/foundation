// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { grpc } from "@namespacelabs.dev-foundation/std-nodejs-grpcgen";
import yargs from "yargs/yargs";

const args = yargs(process.argv)
	.options({
		port: { type: "number" },
	})
	.parse();

export function provideBackend<T>(clientFactory: (...args: any[]) => T): T {
	// TODO: support communication with services in other containers.
	// TODO: support TLS.
	return clientFactory(`127.0.0.1:${args.port}`, grpc.ChannelCredentials.createInsecure());
}
