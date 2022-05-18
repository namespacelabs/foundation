// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation
//
'use strict';
var grpc = require('@grpc/grpc-js');
var languages_nodejs_testdata_services_postuser_service_pb = require('../../../../../languages/nodejs/testdata/services/postuser/service_pb.js');

function serialize_languages_nodejs_testdata_services_postuser_PostUserRequest(arg) {
  if (!(arg instanceof languages_nodejs_testdata_services_postuser_service_pb.PostUserRequest)) {
    throw new Error('Expected argument of type languages.nodejs.testdata.services.postuser.PostUserRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_languages_nodejs_testdata_services_postuser_PostUserRequest(buffer_arg) {
  return languages_nodejs_testdata_services_postuser_service_pb.PostUserRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_languages_nodejs_testdata_services_postuser_PostUserResponse(arg) {
  if (!(arg instanceof languages_nodejs_testdata_services_postuser_service_pb.PostUserResponse)) {
    throw new Error('Expected argument of type languages.nodejs.testdata.services.postuser.PostUserResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_languages_nodejs_testdata_services_postuser_PostUserResponse(buffer_arg) {
  return languages_nodejs_testdata_services_postuser_service_pb.PostUserResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var PostUserServiceService = exports.PostUserServiceService = {
  // grpcurl -plaintext -import-path languages/nodejs/testdata/services/postuser -proto service.proto -d '{"user_name":"Bob"}' 127.0.0.1:40000 languages.nodejs.testdata.services.postuser.PostUserService/GetUserPosts
//
// {
//   "output": "Response: Input: Bob"
// }
getUserPosts: {
    path: '/languages.nodejs.testdata.services.postuser.PostUserService/GetUserPosts',
    requestStream: false,
    responseStream: false,
    requestType: languages_nodejs_testdata_services_postuser_service_pb.PostUserRequest,
    responseType: languages_nodejs_testdata_services_postuser_service_pb.PostUserResponse,
    requestSerialize: serialize_languages_nodejs_testdata_services_postuser_PostUserRequest,
    requestDeserialize: deserialize_languages_nodejs_testdata_services_postuser_PostUserRequest,
    responseSerialize: serialize_languages_nodejs_testdata_services_postuser_PostUserResponse,
    responseDeserialize: deserialize_languages_nodejs_testdata_services_postuser_PostUserResponse,
  },
};

exports.PostUserServiceClient = grpc.makeGenericClientConstructor(PostUserServiceService);
