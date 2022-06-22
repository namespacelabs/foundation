// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { GrpcRegistrar } from "@namespacelabs.dev-foundation/std-nodejs-grpc";
import { ServiceDeps, WireService } from "./deps.fn";
import { bindPostUserServiceServer, PostUserServiceServer } from "./service_grpc.fn";
import { PostUserRequest } from "./service_pb";

export const wireService: WireService = (deps: ServiceDeps, registrar: GrpcRegistrar) => {
	const service: PostUserServiceServer = {
		getUserPosts: async (request: PostUserRequest) => {
			const postResponse = await deps.postService.post({ input: request.userName });
			return { output: `Response: ${postResponse.output}` };
		},
	};

	registrar.registerGrpcService(bindPostUserServiceServer(service));
};
