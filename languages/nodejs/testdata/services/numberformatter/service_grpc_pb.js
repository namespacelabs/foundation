// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation
//
'use strict';
var grpc = require('@grpc/grpc-js');
var services_numberformatter_service_pb = require('../../services/numberformatter/service_pb.js');

function serialize_languages_nodejs_testdata_services_numberformatter_FormatRequest(arg) {
  if (!(arg instanceof services_numberformatter_service_pb.FormatRequest)) {
    throw new Error('Expected argument of type languages.nodejs.testdata.services.numberformatter.FormatRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_languages_nodejs_testdata_services_numberformatter_FormatRequest(buffer_arg) {
  return services_numberformatter_service_pb.FormatRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_languages_nodejs_testdata_services_numberformatter_FormatResponse(arg) {
  if (!(arg instanceof services_numberformatter_service_pb.FormatResponse)) {
    throw new Error('Expected argument of type languages.nodejs.testdata.services.numberformatter.FormatResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_languages_nodejs_testdata_services_numberformatter_FormatResponse(buffer_arg) {
  return services_numberformatter_service_pb.FormatResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// grpcurl -plaintext -import-path languages/nodejs/testdata/services/numberformatter -proto service.proto \
// -d '{"input":12.345445}' 127.0.0.1:XXX \
// languages.nodejs.testdata.services.numberformatter.FormatService/Format
//
// Response:
//
// {
//   "output": [
//     "First instance of the \"batchformatter\" extension:",
//     "  Singleton formatter output: Formatted value: 12.35. This is called 1 times.",
//     "  Scoped formatter output: Formatted value: 12.34544. This is called 1 times.",
//     "Second instance of the \"batchformatter\" extension:",
//     "  Singleton formatter output: Formatted value: 12.35. This is called 2 times.",
//     "  Scoped formatter output: Formatted value: 12.34544. This is called 1 times."
//   ]
// }
var FormatServiceService = exports.FormatServiceService = {
  format: {
    path: '/languages.nodejs.testdata.services.numberformatter.FormatService/Format',
    requestStream: false,
    responseStream: false,
    requestType: services_numberformatter_service_pb.FormatRequest,
    responseType: services_numberformatter_service_pb.FormatResponse,
    requestSerialize: serialize_languages_nodejs_testdata_services_numberformatter_FormatRequest,
    requestDeserialize: deserialize_languages_nodejs_testdata_services_numberformatter_FormatRequest,
    responseSerialize: serialize_languages_nodejs_testdata_services_numberformatter_FormatResponse,
    responseDeserialize: deserialize_languages_nodejs_testdata_services_numberformatter_FormatResponse,
  },
};

exports.FormatServiceClient = grpc.makeGenericClientConstructor(FormatServiceService);
