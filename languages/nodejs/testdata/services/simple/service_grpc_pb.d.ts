// GENERATED CODE -- DO NOT EDIT!

// package: languages.nodejs.testdata.services.simple
// file: services/simple/service.proto

import * as services_simple_service_pb from "../../services/simple/service_pb";
import * as grpc from "@grpc/grpc-js";

interface IPostServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
  post: grpc.MethodDefinition<services_simple_service_pb.PostRequest, services_simple_service_pb.PostResponse>;
}

export const PostServiceService: IPostServiceService;

export interface IPostServiceServer extends grpc.UntypedServiceImplementation {
  post: grpc.handleUnaryCall<services_simple_service_pb.PostRequest, services_simple_service_pb.PostResponse>;
}

export class PostServiceClient extends grpc.Client {
  constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
  post(argument: services_simple_service_pb.PostRequest, callback: grpc.requestCallback<services_simple_service_pb.PostResponse>): grpc.ClientUnaryCall;
  post(argument: services_simple_service_pb.PostRequest, metadataOrOptions: grpc.Metadata | grpc.CallOptions | null, callback: grpc.requestCallback<services_simple_service_pb.PostResponse>): grpc.ClientUnaryCall;
  post(argument: services_simple_service_pb.PostRequest, metadata: grpc.Metadata | null, options: grpc.CallOptions | null, callback: grpc.requestCallback<services_simple_service_pb.PostResponse>): grpc.ClientUnaryCall;
}
