// package: languages.nodejs.testdata.services.simple
// file: services/simple/service.proto

import * as jspb from "google-protobuf";

export class PostRequest extends jspb.Message {
  getInput(): string;
  setInput(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PostRequest.AsObject;
  static toObject(includeInstance: boolean, msg: PostRequest): PostRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PostRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PostRequest;
  static deserializeBinaryFromReader(message: PostRequest, reader: jspb.BinaryReader): PostRequest;
}

export namespace PostRequest {
  export type AsObject = {
    input: string,
  }
}

export class PostResponse extends jspb.Message {
  getOutput(): string;
  setOutput(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PostResponse.AsObject;
  static toObject(includeInstance: boolean, msg: PostResponse): PostResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PostResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PostResponse;
  static deserializeBinaryFromReader(message: PostResponse, reader: jspb.BinaryReader): PostResponse;
}

export namespace PostResponse {
  export type AsObject = {
    output: string,
  }
}
