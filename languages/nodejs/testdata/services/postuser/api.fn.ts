// This file was automatically generated.
// Contains type and function definitions that needs to be implemented in "impl.ts".

import * as impl from "./impl";
import { Registrar } from "@namespacelabs/foundation";
import * as i0 from "@namespacelabs.dev-foundation/std-grpc/internal.fn"
import * as i1 from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/service_grpc_pb"

export interface ServiceDeps {
	postService: i1.PostServiceClient;
}

export type WireService = (deps: ServiceDeps, registrar: Registrar) => void;
export const wireService: WireService = impl.wireService;
