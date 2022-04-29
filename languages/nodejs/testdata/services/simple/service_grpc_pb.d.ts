// GENERATED CODE -- DO NOT EDIT!

// package: nodejsonly.nodejsservice
// file: languages/nodejs/testdata/services/simple/service.proto

import * as languages_nodejs_testdata_services_simple_service_pb from "../../../../../languages/nodejs/testdata/services/simple/service_pb";
import * as grpc from "@grpc/grpc-js";

interface IPostServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
  post: grpc.MethodDefinition<languages_nodejs_testdata_services_simple_service_pb.PostRequest, languages_nodejs_testdata_services_simple_service_pb.PostResponse>;
}

export const PostServiceService: IPostServiceService;

export interface IPostServiceServer extends grpc.UntypedServiceImplementation {
  post: grpc.handleUnaryCall<languages_nodejs_testdata_services_simple_service_pb.PostRequest, languages_nodejs_testdata_services_simple_service_pb.PostResponse>;
}

export class PostServiceClient extends grpc.Client {
  constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
  post(argument: languages_nodejs_testdata_services_simple_service_pb.PostRequest, callback: grpc.requestCallback<languages_nodejs_testdata_services_simple_service_pb.PostResponse>): grpc.ClientUnaryCall;
  post(argument: languages_nodejs_testdata_services_simple_service_pb.PostRequest, metadataOrOptions: grpc.Metadata | grpc.CallOptions | null, callback: grpc.requestCallback<languages_nodejs_testdata_services_simple_service_pb.PostResponse>): grpc.ClientUnaryCall;
  post(argument: languages_nodejs_testdata_services_simple_service_pb.PostRequest, metadata: grpc.Metadata | null, options: grpc.CallOptions | null, callback: grpc.requestCallback<languages_nodejs_testdata_services_simple_service_pb.PostResponse>): grpc.ClientUnaryCall;
}
