// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { ChannelCredentials } from "@grpc/grpc-js";
import { Backend } from "./protos/provider_pb";
import yargs from "yargs/yargs";

const args = yargs(process.argv)
	.options({
		port: { type: "number" },
	})
	.parse();

export function provideBackend<T>(unused: Backend, outputType: new (...args: any[]) => T): T {
	// TODO: support communication with services in other containers.
	// TODO: support TLS.
	return new outputType(`127.0.0.1:${args.port}`, ChannelCredentials.createInsecure(), {});
}
