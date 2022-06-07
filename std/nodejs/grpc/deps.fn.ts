// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import {GrpcRegistrar} from "@namespacelabs.dev-foundation/std-nodejs-grpc"
import * as i0 from "@namespacelabs.dev-foundation/std-nodejs-grpc/provider_pb";
import * as i1 from "@namespacelabs.dev-foundation/std-nodejs-grpc/interceptor";
import * as i2 from "@namespacelabs.dev-foundation/std-nodejs-grpc/registrar";



export const Package = {
	name: "namespacelabs.dev/foundation/std/nodejs/grpc",
};

export const TransitiveInitializers: Initializer[] = [
];


export const GrpcInterceptorRegistrarProvider = (graph: DependencyGraph, input: i0.NoArgs) =>
	provideGrpcInterceptorRegistrar(
		input
	);

export type ProvideGrpcInterceptorRegistrar = (input: i0.NoArgs) =>
		i1.GrpcInterceptorRegistrar;
export const provideGrpcInterceptorRegistrar: ProvideGrpcInterceptorRegistrar = impl.provideGrpcInterceptorRegistrar;


export const GrpcRegistrarProvider = (graph: DependencyGraph, input: i0.NoArgs) =>
	provideGrpcRegistrar(
		input
	);

export type ProvideGrpcRegistrar = (input: i0.NoArgs) =>
		i2.GrpcRegistrar;
export const provideGrpcRegistrar: ProvideGrpcRegistrar = impl.provideGrpcRegistrar;
