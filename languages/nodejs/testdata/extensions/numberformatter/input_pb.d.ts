// package: languages.nodejs.testdata.extensions.numberformatter
// file: languages/nodejs/testdata/extensions/numberformatter/input.proto

import * as jspb from "google-protobuf";

export class FormattingSettings extends jspb.Message {
  getPrecision(): number;
  setPrecision(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): FormattingSettings.AsObject;
  static toObject(includeInstance: boolean, msg: FormattingSettings): FormattingSettings.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: FormattingSettings, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): FormattingSettings;
  static deserializeBinaryFromReader(message: FormattingSettings, reader: jspb.BinaryReader): FormattingSettings;
}

export namespace FormattingSettings {
  export type AsObject = {
    precision: number,
  }
}

