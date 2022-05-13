// GENERATED CODE -- DO NOT EDIT!

// package: languages.nodejs.testdata.services.postuser
// file: languages/nodejs/testdata/services/postuser/service.proto

import * as languages_nodejs_testdata_services_postuser_service_pb from "../../../../../languages/nodejs/testdata/services/postuser/service_pb";
import * as grpc from "@grpc/grpc-js";

interface IPostUserServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
  getUserPosts: grpc.MethodDefinition<languages_nodejs_testdata_services_postuser_service_pb.PostUserRequest, languages_nodejs_testdata_services_postuser_service_pb.PostUserResponse>;
}

export const PostUserServiceService: IPostUserServiceService;

export interface IPostUserServiceServer extends grpc.UntypedServiceImplementation {
  getUserPosts: grpc.handleUnaryCall<languages_nodejs_testdata_services_postuser_service_pb.PostUserRequest, languages_nodejs_testdata_services_postuser_service_pb.PostUserResponse>;
}

export class PostUserServiceClient extends grpc.Client {
  constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
  getUserPosts(argument: languages_nodejs_testdata_services_postuser_service_pb.PostUserRequest, callback: grpc.requestCallback<languages_nodejs_testdata_services_postuser_service_pb.PostUserResponse>): grpc.ClientUnaryCall;
  getUserPosts(argument: languages_nodejs_testdata_services_postuser_service_pb.PostUserRequest, metadataOrOptions: grpc.Metadata | grpc.CallOptions | null, callback: grpc.requestCallback<languages_nodejs_testdata_services_postuser_service_pb.PostUserResponse>): grpc.ClientUnaryCall;
  getUserPosts(argument: languages_nodejs_testdata_services_postuser_service_pb.PostUserRequest, metadata: grpc.Metadata | null, options: grpc.CallOptions | null, callback: grpc.requestCallback<languages_nodejs_testdata_services_postuser_service_pb.PostUserResponse>): grpc.ClientUnaryCall;
}
