// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import {GrpcRegistrar} from "@namespacelabs.dev-foundation/std-nodejs-grpc"



export const Package = {
	name: "namespacelabs.dev/foundation/std/grpc",
};

export const TransitiveInitializers: Initializer[] = [
];


export const BackendProvider = <T>(graph: DependencyGraph, outputTypeCtr: new (...args: any[]) => T) =>
	provideBackend(
		outputTypeCtr
	);

export type ProvideBackend = <T>(outputTypeCtr: new (...args: any[]) => T) =>
		T;
export const provideBackend: ProvideBackend = impl.provideBackend;
