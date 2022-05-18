// package: foundation.std.grpc.protos
// file: std/grpc/protos/provider.proto

import * as jspb from "google-protobuf";
import * as std_proto_options_pb from "../../../std/proto/options_pb";

export class Backend extends jspb.Message {
  getPackageName(): string;
  setPackageName(value: string): void;

  getServiceName(): string;
  setServiceName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Backend.AsObject;
  static toObject(includeInstance: boolean, msg: Backend): Backend.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Backend, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Backend;
  static deserializeBinaryFromReader(message: Backend, reader: jspb.BinaryReader): Backend;
}

export namespace Backend {
  export type AsObject = {
    packageName: string,
    serviceName: string,
  }
}
