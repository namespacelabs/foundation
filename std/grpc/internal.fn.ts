// This file was automatically generated.
// Contains Foundation-internal wiring, the user doesn't interact directly with it.

import * as impl from "./impl";
import * as api from "./api.fn";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";

export const Package = {
  name: "namespacelabs.dev/foundation/std/grpc",
};

export const TransitiveInitializers: Initializer[] = [
];

export const BackendProvider = <T>(graph: DependencyGraph, outputTypeCtr: new (...args: any[]) => T) =>
	api.provideBackend(
		outputTypeCtr
  );
