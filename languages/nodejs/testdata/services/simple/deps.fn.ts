// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs.dev/foundation/std/nodejs/runtime";
import {GrpcRegistrar} from "@namespacelabs.dev/foundation/std/nodejs/grpc"



export const Package = {
	name: "namespacelabs.dev/foundation/languages/nodejs/testdata/services/simple",
};

export const TransitiveInitializers: Initializer[] = [
];

export type WireService = (registrar: GrpcRegistrar) => Promise<void> | void;
export const wireService: WireService = impl.wireService;
