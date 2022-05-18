// package: languages.nodejs.testdata.services.postuser
// file: languages/nodejs/testdata/services/postuser/service.proto

import * as jspb from "google-protobuf";

export class PostUserRequest extends jspb.Message {
  getUserName(): string;
  setUserName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PostUserRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PostUserRequest): PostUserRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PostUserRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PostUserRequest;
  static deserializeBinaryFromReader(message: PostUserRequest, reader: jspb.BinaryReader): PostUserRequest;
}

export namespace PostUserRequest {
  export type AsObject = {
    userName: string,
  }
}

export class PostUserResponse extends jspb.Message {
  getOutput(): string;
  setOutput(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PostUserResponse.AsObject;
  static toObject(includeInstance: boolean, msg: PostUserResponse): PostUserResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PostUserResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PostUserResponse;
  static deserializeBinaryFromReader(message: PostUserResponse, reader: jspb.BinaryReader): PostUserResponse;
}

export namespace PostUserResponse {
  export type AsObject = {
    output: string,
  }
}

