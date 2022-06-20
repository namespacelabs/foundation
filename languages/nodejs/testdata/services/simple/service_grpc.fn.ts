// This file was automatically generated.

import {grpc} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen";
import * as i0 from "./service_pb";
import {adaptClient, CallOptions} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/client";
import {CallContext} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/server";

// API

// PostService - Client

export interface PostServiceClient {
	post(request: i0.PostRequest, options?: CallOptions): Promise<i0.PostResponse>;
}

export function newPostServiceClient(address: string, credentials: grpc.ChannelCredentials, options?: object): PostServiceClient {
	return adaptClient(wrappedPostServiceClientConstructor, address, credentials, options) as PostServiceClient;
}

// PostService - Server

export interface PostServiceServer {
	post(request: i0.PostRequest, context: CallContext): Promise<i0.PostResponse>;
}

export function bindPostServiceServer(server: PostServiceServer) {
	return {
		impl: server,
		definition: PostServiceDefinition,
	}
}

// Wiring

// PostService

const PostServiceDefinition: grpc.ServiceDefinition = {
	post: {
		path: "/languages.nodejs.testdata.services.simple.PostService/Post",
		originalName: "Post",
		requestStream: false,
		responseStream: false,
		requestSerialize: (arg) => Buffer.from(i0.PostRequest.toBinary(arg)),
		requestDeserialize: (arg) => i0.PostRequest.fromBinary(new Uint8Array(arg)),
		responseSerialize: (arg) => Buffer.from(i0.PostResponse.toBinary(arg)),
		responseDeserialize: (arg) => i0.PostResponse.fromBinary(new Uint8Array(arg)),
	},
};

const wrappedPostServiceClientConstructor = grpc.makeGenericClientConstructor(PostServiceDefinition, "Unused service name");

