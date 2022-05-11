// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import * as i0 from "@grpc/grpc-js"



export const Package = {
  name: "namespacelabs.dev/foundation-nodejs-testdata/services/simple",
};

export const TransitiveInitializers: Initializer[] = [
];

export type WireService = (server: i0.Server) => void;
export const wireService: WireService = impl.wireService;
