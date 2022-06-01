// package: std.nodejs.http
// file: std/nodejs/http/provider.proto

import * as jspb from "google-protobuf";

export class NoArgs extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): NoArgs.AsObject;
  static toObject(includeInstance: boolean, msg: NoArgs): NoArgs.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: NoArgs, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): NoArgs;
  static deserializeBinaryFromReader(message: NoArgs, reader: jspb.BinaryReader): NoArgs;
}

export namespace NoArgs {
  export type AsObject = {
  }
}

