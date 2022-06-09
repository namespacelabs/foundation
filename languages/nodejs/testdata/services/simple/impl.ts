// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { GrpcRegistrar } from "@namespacelabs.dev-foundation/std-nodejs-grpc";
import { WireService } from "./deps.fn";
import { bindPostServiceServer } from "./service_grpc.fn";
import { PostRequest, PostResponse } from "./service_pb";

export const wireService: WireService = async (registrar: GrpcRegistrar) =>
	registrar.registerGrpcService(
		bindPostServiceServer({
			post: async (request: PostRequest): Promise<PostResponse> => {
				const response: PostResponse = new PostResponse();
				response.setOutput(`Input: ${request.getInput()}`);
				return response;
			},
		})
	);
