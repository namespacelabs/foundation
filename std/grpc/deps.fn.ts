// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-grpc/protos/provider_pb"



export const Package = {
  name: "namespacelabs.dev/foundation/std/grpc",
};

export const TransitiveInitializers: Initializer[] = [
];


export const BackendProvider = <T>(graph: DependencyGraph, input: i0.Backend, outputTypeCtr: new (...args: any[]) => T) =>
	provideBackend(
		input, outputTypeCtr
  );

export type ProvideBackend = <T>(input: i0.Backend, outputTypeCtr: new (...args: any[]) => T) =>
		T;
export const provideBackend: ProvideBackend = impl.provideBackend;
