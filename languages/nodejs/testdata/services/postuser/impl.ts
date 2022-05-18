// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { sendUnaryData, ServerUnaryCall } from "@grpc/grpc-js";
import { PostRequest } from "@namespacelabs.dev-foundation/languages-nodejs-testdata-services-simple/service_pb";
import { Registrar } from "@namespacelabs/foundation";
import { ServiceDeps, WireService } from "./deps.fn";
import { IPostUserServiceServer, PostUserServiceService } from "./service_grpc_pb";
import { PostUserRequest, PostUserResponse } from "./service_pb";

export const wireService: WireService = (deps: ServiceDeps, registrar: Registrar): void => {
	const service: IPostUserServiceServer = {
		getUserPosts: function (
			call: ServerUnaryCall<PostUserRequest, PostUserResponse>,
			callback: sendUnaryData<PostUserResponse>
		): void {
			const request = new PostRequest();
			request.setInput(call.request.getUserName());
			deps.postService.post(request, (err, postResponse) => {
				if (!postResponse) {
					callback(err, null);
				} else {
					const response: PostUserResponse = new PostUserResponse();
					response.setOutput(`Response: ${postResponse.getOutput()}`);

					callback(null, response);
				}
			});
		},
	};

	registrar.registerGrpcService(PostUserServiceService, service);
};
