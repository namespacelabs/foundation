// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs.dev/foundation/std/nodejs/runtime";
import {GrpcRegistrar} from "@namespacelabs.dev/foundation/std/nodejs/grpc"



export const Package = {
	name: "namespacelabs.dev/foundation/std/grpc",
};

export const TransitiveInitializers: Initializer[] = [
];


export const BackendProvider = <T>(graph: DependencyGraph, outputTypeFactory: (...args: any[]) => T) =>
	provideBackend(
		outputTypeFactory
	);

export type ProvideBackend = <T>(outputTypeFactory: (...args: any[]) => T) =>
		T;
export const provideBackend: ProvideBackend = impl.provideBackend;
