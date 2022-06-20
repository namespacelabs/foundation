// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer, InstantiationContext } from "@namespacelabs.dev/foundation/std/nodejs/runtime";
import {GrpcRegistrar} from "@namespacelabs.dev/foundation/std/nodejs/grpc"
import * as i0 from "@namespacelabs.dev/foundation/std/grpc/protos/provider_pb";



export const Package = {
	name: "namespacelabs.dev/foundation/std/grpc",
};

export const TransitiveInitializers: Initializer[] = [
];


export const BackendProvider = <T>(
	  graph: DependencyGraph,
	  input: i0.Backend,
		outputTypeFactory: (...args: any[]) => T,
	  context: InstantiationContext) =>
	provideBackend(
		input,outputTypeFactory,
		context
	);

export type ProvideBackend = <T>(input: i0.Backend, outputTypeFactory: (...args: any[]) => T, context: InstantiationContext) =>
		T;
export const provideBackend: ProvideBackend = impl.provideBackend;
