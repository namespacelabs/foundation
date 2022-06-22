// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer, InstantiationContext } from "@namespacelabs.dev/foundation/std/nodejs/runtime";
import {GrpcRegistrar} from "@namespacelabs.dev/foundation/std/nodejs/grpc"
import * as i0 from "@namespacelabs.dev/foundation/std/nodejs/http/provider_pb";
import * as i1 from "@namespacelabs.dev/foundation/std/nodejs/http/httpserver";



export const Package = {
	name: "namespacelabs.dev/foundation/std/nodejs/http",
};

export const TransitiveInitializers: Initializer[] = [
];


export const HttpServerProvider = (
	  graph: DependencyGraph,
	  input: i0.NoArgs,
	  context: InstantiationContext) =>
	provideHttpServer(
		input,
		context
	);

export type ProvideHttpServer = (input: i0.NoArgs, context: InstantiationContext) =>
		Promise<i1.HttpServer>;
export const provideHttpServer: ProvideHttpServer = impl.provideHttpServer;
