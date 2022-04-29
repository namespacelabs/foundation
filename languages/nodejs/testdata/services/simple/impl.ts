import { sendUnaryData, Server, ServerUnaryCall } from "@grpc/grpc-js";
import { WireService } from "./deps.fn";
import { IPostServiceServer, PostServiceService } from "./service_grpc_pb";
import { PostRequest, PostResponse } from "./service_pb";

export const wireService: WireService = (server: Server): void => {
	const service: IPostServiceServer = {
		post: function (
			call: ServerUnaryCall<PostRequest, PostResponse>,
			callback: sendUnaryData<PostResponse>
		): void {
			const response: PostResponse = new PostResponse();
			response.setOutput(`Input: ${call.request.getInput()}`);

			callback(null, response);
		},
	};

	server.addService(PostServiceService, service);
};
