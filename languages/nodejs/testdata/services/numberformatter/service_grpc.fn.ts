// This file was automatically generated.

import {grpc} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen";
import * as i0 from "./service_pb";
import {adaptClient, CallOptions} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/client";
import {CallContext} from "@namespacelabs.dev-foundation/std-nodejs-grpcgen/server";

// API

// FormatService - Client

export interface FormatServiceClient {
	format(request: i0.FormatRequest, options?: CallOptions): Promise<i0.FormatResponse>;
}

export function newFormatServiceClient(address: string, credentials: grpc.ChannelCredentials, options?: object): FormatServiceClient {
	return adaptClient(wrappedFormatServiceClientConstructor, address, credentials, options) as FormatServiceClient;
}

// FormatService - Server

export interface FormatServiceServer {
	format(request: i0.FormatRequest, context: CallContext): Promise<i0.FormatResponse>;
}

export function bindFormatServiceServer(server: FormatServiceServer) {
	return {
		impl: server,
		definition: FormatServiceDefinition,
	}
}

// Wiring

// FormatService

const FormatServiceDefinition: grpc.ServiceDefinition = {
	format: {
		path: "/languages.nodejs.testdata.services.numberformatter.FormatService/Format",
		originalName: "Format",
		requestStream: false,
		responseStream: false,
		requestSerialize: (arg) => Buffer.from(arg.serializeBinary()),
		requestDeserialize: (arg) => i0.FormatRequest.deserializeBinary(new Uint8Array(arg)),
		responseSerialize: (arg) => Buffer.from(arg.serializeBinary()),
		responseDeserialize: (arg) => i0.FormatResponse.deserializeBinary(new Uint8Array(arg)),
	},
};

const wrappedFormatServiceClientConstructor = grpc.makeGenericClientConstructor(FormatServiceDefinition, "Unused service name");

