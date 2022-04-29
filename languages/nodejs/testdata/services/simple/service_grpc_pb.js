// GENERATED CODE -- DO NOT EDIT!

'use strict';
var grpc = require('@grpc/grpc-js');
var languages_nodejs_testdata_services_simple_service_pb = require('../../../../../languages/nodejs/testdata/services/simple/service_pb.js');

function serialize_nodejsonly_nodejsservice_PostRequest(arg) {
  if (!(arg instanceof languages_nodejs_testdata_services_simple_service_pb.PostRequest)) {
    throw new Error('Expected argument of type nodejsonly.nodejsservice.PostRequest');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_nodejsonly_nodejsservice_PostRequest(buffer_arg) {
  return languages_nodejs_testdata_services_simple_service_pb.PostRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_nodejsonly_nodejsservice_PostResponse(arg) {
  if (!(arg instanceof languages_nodejs_testdata_services_simple_service_pb.PostResponse)) {
    throw new Error('Expected argument of type nodejsonly.nodejsservice.PostResponse');
  }
  return Buffer.from(arg.serializeBinary());
}

function deserialize_nodejsonly_nodejsservice_PostResponse(buffer_arg) {
  return languages_nodejs_testdata_services_simple_service_pb.PostResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


var PostServiceService = exports.PostServiceService = {
  post: {
    path: '/nodejsonly.nodejsservice.PostService/Post',
    requestStream: false,
    responseStream: false,
    requestType: languages_nodejs_testdata_services_simple_service_pb.PostRequest,
    responseType: languages_nodejs_testdata_services_simple_service_pb.PostResponse,
    requestSerialize: serialize_nodejsonly_nodejsservice_PostRequest,
    requestDeserialize: deserialize_nodejsonly_nodejsservice_PostRequest,
    responseSerialize: serialize_nodejsonly_nodejsservice_PostResponse,
    responseDeserialize: deserialize_nodejsonly_nodejsservice_PostResponse,
  },
};

exports.PostServiceClient = grpc.makeGenericClientConstructor(PostServiceService);
