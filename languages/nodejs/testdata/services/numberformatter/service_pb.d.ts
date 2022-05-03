// package: languages.nodejs.testdata.services.numberformatter
// file: languages/nodejs/testdata/services/numberformatter/service.proto

import * as jspb from "google-protobuf";

export class FormatRequest extends jspb.Message {
  getInput(): number;
  setInput(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): FormatRequest.AsObject;
  static toObject(includeInstance: boolean, msg: FormatRequest): FormatRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: FormatRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): FormatRequest;
  static deserializeBinaryFromReader(message: FormatRequest, reader: jspb.BinaryReader): FormatRequest;
}

export namespace FormatRequest {
  export type AsObject = {
    input: number,
  }
}

export class FormatResponse extends jspb.Message {
  clearOutputList(): void;
  getOutputList(): Array<string>;
  setOutputList(value: Array<string>): void;
  addOutput(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): FormatResponse.AsObject;
  static toObject(includeInstance: boolean, msg: FormatResponse): FormatResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: FormatResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): FormatResponse;
  static deserializeBinaryFromReader(message: FormatResponse, reader: jspb.BinaryReader): FormatResponse;
}

export namespace FormatResponse {
  export type AsObject = {
    outputList: Array<string>,
  }
}

