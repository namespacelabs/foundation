// This file was automatically generated.

import {grpc} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen";
import * as i0 from "./service_pb";
import {adaptClient, CallOptions} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/client";
import {CallContext} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/server";

// API

// PostUserService - Client

export interface PostUserServiceClient {
	getUserPosts(request: i0.PostUserRequest, options?: CallOptions): Promise<i0.PostUserResponse>;
}

export function newPostUserServiceClient(address: string, credentials: grpc.ChannelCredentials, options?: object): PostUserServiceClient {
	return adaptClient(wrappedPostUserServiceClientConstructor, address, credentials, options) as PostUserServiceClient;
}

// PostUserService - Server

export interface PostUserServiceServer {
	getUserPosts(request: i0.PostUserRequest, context: CallContext): Promise<i0.PostUserResponse>;
}

export function bindPostUserServiceServer(server: PostUserServiceServer) {
	return {
		impl: server,
		definition: PostUserServiceDefinition,
	}
}

// Wiring

// PostUserService

const PostUserServiceDefinition: grpc.ServiceDefinition = {
	getUserPosts: {
		path: "/languages.nodejs.testdata.services.postuser.PostUserService/GetUserPosts",
		originalName: "GetUserPosts",
		requestStream: false,
		responseStream: false,
		requestSerialize: (arg) => Buffer.from(arg.serializeBinary()),
		requestDeserialize: (arg) => i0.PostUserRequest.deserializeBinary(new Uint8Array(arg)),
		responseSerialize: (arg) => Buffer.from(arg.serializeBinary()),
		responseDeserialize: (arg) => i0.PostUserResponse.deserializeBinary(new Uint8Array(arg)),
	},
};

const wrappedPostUserServiceClientConstructor = grpc.makeGenericClientConstructor(PostUserServiceDefinition, "Unused service name");

