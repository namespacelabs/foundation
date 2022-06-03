// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-nodejs-grpc/provider_pb";
import * as i1 from "@namespacelabs.dev-foundation/std-nodejs-grpc/registrar";



export const Package = {
	name: "namespacelabs.dev/foundation/std/nodejs/grpc",
};

export const TransitiveInitializers: Initializer[] = [
];


export const GrpcRegistrarProvider = (graph: DependencyGraph, input: i0.NoArgs) =>
	provideGrpcRegistrar(
		input
	);

export type ProvideGrpcRegistrar = (input: i0.NoArgs) =>
		i1.GrpcRegistrar;
export const provideGrpcRegistrar: ProvideGrpcRegistrar = impl.provideGrpcRegistrar;
