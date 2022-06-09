// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { PostRequest } from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/service_pb";
import { GrpcRegistrar } from "@namespacelabs.dev-foundation/std-nodejs-grpc";
import { ServiceDeps, WireService } from "./deps.fn";
import { bindPostUserServiceServer, PostUserServiceServer } from "./service_grpc.fn";
import { PostUserRequest, PostUserResponse } from "./service_pb";

export const wireService: WireService = (deps: ServiceDeps, registrar: GrpcRegistrar) => {
	const service: PostUserServiceServer = {
		getUserPosts: async (request: PostUserRequest) => {
			const postRequest = new PostRequest();
			postRequest.setInput(request.getUserName());
			const postResponse = await deps.postService.post(postRequest);
			const response: PostUserResponse = new PostUserResponse();
			response.setOutput(`Response: ${postResponse.getOutput()}`);
			return response;
		},
	};

	registrar.registerGrpcService(bindPostUserServiceServer(service));
};
