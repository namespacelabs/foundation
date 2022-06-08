// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import {GrpcRegistrar} from "@namespacelabs.dev-foundation/std-nodejs-grpc"



export const Package = {
	name: "namespacelabs.dev/foundation/std/nodejs/grpcgen",
};

const initializer = {
	package: Package,
	initialize: impl.initialize,
};

export type Prepare = () => Promise<void> | void;
export const prepare: Prepare = impl.initialize;

export const TransitiveInitializers: Initializer[] = [
	initializer,
];
