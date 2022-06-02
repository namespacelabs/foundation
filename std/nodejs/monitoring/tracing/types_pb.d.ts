// package: foundation.std.nodejs.monitoring.tracing
// file: std/nodejs/monitoring/tracing/types.proto

import * as jspb from "google-protobuf";

export class ExporterArgs extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ExporterArgs.AsObject;
  static toObject(includeInstance: boolean, msg: ExporterArgs): ExporterArgs.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ExporterArgs, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ExporterArgs;
  static deserializeBinaryFromReader(message: ExporterArgs, reader: jspb.BinaryReader): ExporterArgs;
}

export namespace ExporterArgs {
  export type AsObject = {
    name: string,
  }
}

