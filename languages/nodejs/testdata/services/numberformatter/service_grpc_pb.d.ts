// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// GENERATED CODE -- DO NOT EDIT!

// package: languages.nodejs.testdata.services.numberformatter
// file: languages/nodejs/testdata/services/numberformatter/service.proto

import * as languages_nodejs_testdata_services_numberformatter_service_pb from "../../../../../languages/nodejs/testdata/services/numberformatter/service_pb";
import * as grpc from "@grpc/grpc-js";

interface IFormatServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
  format: grpc.MethodDefinition<languages_nodejs_testdata_services_numberformatter_service_pb.FormatRequest, languages_nodejs_testdata_services_numberformatter_service_pb.FormatResponse>;
}

export const FormatServiceService: IFormatServiceService;

export interface IFormatServiceServer extends grpc.UntypedServiceImplementation {
  format: grpc.handleUnaryCall<languages_nodejs_testdata_services_numberformatter_service_pb.FormatRequest, languages_nodejs_testdata_services_numberformatter_service_pb.FormatResponse>;
}

export class FormatServiceClient extends grpc.Client {
  constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
  format(argument: languages_nodejs_testdata_services_numberformatter_service_pb.FormatRequest, callback: grpc.requestCallback<languages_nodejs_testdata_services_numberformatter_service_pb.FormatResponse>): grpc.ClientUnaryCall;
  format(argument: languages_nodejs_testdata_services_numberformatter_service_pb.FormatRequest, metadataOrOptions: grpc.Metadata | grpc.CallOptions | null, callback: grpc.requestCallback<languages_nodejs_testdata_services_numberformatter_service_pb.FormatResponse>): grpc.ClientUnaryCall;
  format(argument: languages_nodejs_testdata_services_numberformatter_service_pb.FormatRequest, metadata: grpc.Metadata | null, options: grpc.CallOptions | null, callback: grpc.requestCallback<languages_nodejs_testdata_services_numberformatter_service_pb.FormatResponse>): grpc.ClientUnaryCall;
}
