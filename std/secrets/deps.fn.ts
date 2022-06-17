// This file was automatically generated.

import * as impl from "./impl";
import { DependencyGraph, Initializer } from "@namespacelabs/foundation";
import {GrpcRegistrar} from "@namespacelabs.dev-foundation/std-nodejs-grpc"
import * as i0 from "@namespacelabs.dev-foundation/std-secrets/provider_pb";



export const Package = {
	name: "namespacelabs.dev/foundation/std/secrets",
};

export const TransitiveInitializers: Initializer[] = [
];


export const SecretProvider = (graph: DependencyGraph, input: i0.Secret) =>
	provideSecret(
		input
	);

export type ProvideSecret = (input: i0.Secret) =>
		Promise<i0.Value>;
export const provideSecret: ProvideSecret = impl.provideSecret;
