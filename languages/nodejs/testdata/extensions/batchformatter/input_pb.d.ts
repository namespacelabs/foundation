// package: languages.nodejs.testdata.extensions.batchformatter
// file: extensions/batchformatter/input.proto

import * as jspb from "google-protobuf";

export class InputData extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): InputData.AsObject;
  static toObject(includeInstance: boolean, msg: InputData): InputData.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: InputData, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): InputData;
  static deserializeBinaryFromReader(message: InputData, reader: jspb.BinaryReader): InputData;
}

export namespace InputData {
  export type AsObject = {
  }
}
