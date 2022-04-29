// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// GENERATED CODE -- DO NOT EDIT!

'use strict';
var grpc = require('@grpc/grpc-js');
var languages_nodejs_testdata_services_simple_service_pb = require('../../../../../languages/nodejs/testdata/services/simple/service_pb.js');

function serialize_languages_nodejs_testdata_services_simple_PostRequest(arg) {
  if (!(arg instanceof languages_nodejs_testdata_services_simple_service_pb.PostRequest)) {
    throw new Error('Expected argument of type languages.nodejs.testdata.services.simple.PostRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_languages_nodejs_testdata_services_simple_PostRequest(buffer_arg) {
  return languages_nodejs_testdata_services_simple_service_pb.PostRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_languages_nodejs_testdata_services_simple_PostResponse(arg) {
  if (!(arg instanceof languages_nodejs_testdata_services_simple_service_pb.PostResponse)) {
    throw new Error('Expected argument of type languages.nodejs.testdata.services.simple.PostResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_languages_nodejs_testdata_services_simple_PostResponse(buffer_arg) {
  return languages_nodejs_testdata_services_simple_service_pb.PostResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var PostServiceService = exports.PostServiceService = {
  post: {
    path: '/languages.nodejs.testdata.services.simple.PostService/Post',
    requestStream: false,
    responseStream: false,
    requestType: languages_nodejs_testdata_services_simple_service_pb.PostRequest,
    responseType: languages_nodejs_testdata_services_simple_service_pb.PostResponse,
    requestSerialize: serialize_languages_nodejs_testdata_services_simple_PostRequest,
    requestDeserialize: deserialize_languages_nodejs_testdata_services_simple_PostRequest,
    responseSerialize: serialize_languages_nodejs_testdata_services_simple_PostResponse,
    responseDeserialize: deserialize_languages_nodejs_testdata_services_simple_PostResponse,
  },
};

exports.PostServiceClient = grpc.makeGenericClientConstructor(PostServiceService);
