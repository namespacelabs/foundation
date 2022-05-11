// GENERATED CODE -- DO NOT EDIT!

// package: languages.nodejs.testdata.services.numberformatter
// file: services/numberformatter/service.proto

import * as services_numberformatter_service_pb from "../../services/numberformatter/service_pb";
import * as grpc from "@grpc/grpc-js";

interface IFormatServiceService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
  format: grpc.MethodDefinition<services_numberformatter_service_pb.FormatRequest, services_numberformatter_service_pb.FormatResponse>;
}

export const FormatServiceService: IFormatServiceService;

export interface IFormatServiceServer extends grpc.UntypedServiceImplementation {
  format: grpc.handleUnaryCall<services_numberformatter_service_pb.FormatRequest, services_numberformatter_service_pb.FormatResponse>;
}

export class FormatServiceClient extends grpc.Client {
  constructor(address: string, credentials: grpc.ChannelCredentials, options?: object);
  format(argument: services_numberformatter_service_pb.FormatRequest, callback: grpc.requestCallback<services_numberformatter_service_pb.FormatResponse>): grpc.ClientUnaryCall;
  format(argument: services_numberformatter_service_pb.FormatRequest, metadataOrOptions: grpc.Metadata | grpc.CallOptions | null, callback: grpc.requestCallback<services_numberformatter_service_pb.FormatResponse>): grpc.ClientUnaryCall;
  format(argument: services_numberformatter_service_pb.FormatRequest, metadata: grpc.Metadata | null, options: grpc.CallOptions | null, callback: grpc.requestCallback<services_numberformatter_service_pb.FormatResponse>): grpc.ClientUnaryCall;
}
