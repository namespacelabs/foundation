// package: foundation.std.secrets
// file: std/secrets/provider.proto

import * as jspb from "google-protobuf";

export class Secrets extends jspb.Message {
  clearSecretList(): void;
  getSecretList(): Array<Secret>;
  setSecretList(value: Array<Secret>): void;
  addSecret(value?: Secret, index?: number): Secret;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Secrets.AsObject;
  static toObject(includeInstance: boolean, msg: Secrets): Secrets.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Secrets, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Secrets;
  static deserializeBinaryFromReader(message: Secrets, reader: jspb.BinaryReader): Secrets;
}

export namespace Secrets {
  export type AsObject = {
    secretList: Array<Secret.AsObject>,
  }
}

export class Secret extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasGenerate(): boolean;
  clearGenerate(): void;
  getGenerate(): GenerateSpecification | undefined;
  setGenerate(value?: GenerateSpecification): void;

  getOptional(): boolean;
  setOptional(value: boolean): void;

  getExperimentalMountAsEnvVar(): string;
  setExperimentalMountAsEnvVar(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Secret.AsObject;
  static toObject(includeInstance: boolean, msg: Secret): Secret.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Secret, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Secret;
  static deserializeBinaryFromReader(message: Secret, reader: jspb.BinaryReader): Secret;
}

export namespace Secret {
  export type AsObject = {
    name: string,
    generate?: GenerateSpecification.AsObject,
    optional: boolean,
    experimentalMountAsEnvVar: string,
  }
}

export class GenerateSpecification extends jspb.Message {
  getUniqueId(): string;
  setUniqueId(value: string): void;

  getRandomByteCount(): number;
  setRandomByteCount(value: number): void;

  getFormat(): GenerateSpecification.FormatMap[keyof GenerateSpecification.FormatMap];
  setFormat(value: GenerateSpecification.FormatMap[keyof GenerateSpecification.FormatMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GenerateSpecification.AsObject;
  static toObject(includeInstance: boolean, msg: GenerateSpecification): GenerateSpecification.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GenerateSpecification, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GenerateSpecification;
  static deserializeBinaryFromReader(message: GenerateSpecification, reader: jspb.BinaryReader): GenerateSpecification;
}

export namespace GenerateSpecification {
  export type AsObject = {
    uniqueId: string,
    randomByteCount: number,
    format: GenerateSpecification.FormatMap[keyof GenerateSpecification.FormatMap],
  }

  export interface FormatMap {
    FORMAT_UNKNOWN: 0;
    FORMAT_BASE64: 1;
    FORMAT_BASE32: 2;
  }

  export const Format: FormatMap;
}

export class SecretDevMap extends jspb.Message {
  clearConfigureList(): void;
  getConfigureList(): Array<SecretDevMap.Configure>;
  setConfigureList(value: Array<SecretDevMap.Configure>): void;
  addConfigure(value?: SecretDevMap.Configure, index?: number): SecretDevMap.Configure;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SecretDevMap.AsObject;
  static toObject(includeInstance: boolean, msg: SecretDevMap): SecretDevMap.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SecretDevMap, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SecretDevMap;
  static deserializeBinaryFromReader(message: SecretDevMap, reader: jspb.BinaryReader): SecretDevMap;
}

export namespace SecretDevMap {
  export type AsObject = {
    configureList: Array<SecretDevMap.Configure.AsObject>,
  }

  export class Configure extends jspb.Message {
    getPackageName(): string;
    setPackageName(value: string): void;

    clearSecretList(): void;
    getSecretList(): Array<SecretDevMap.SecretSpec>;
    setSecretList(value: Array<SecretDevMap.SecretSpec>): void;
    addSecret(value?: SecretDevMap.SecretSpec, index?: number): SecretDevMap.SecretSpec;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Configure.AsObject;
    static toObject(includeInstance: boolean, msg: Configure): Configure.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Configure, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Configure;
    static deserializeBinaryFromReader(message: Configure, reader: jspb.BinaryReader): Configure;
  }

  export namespace Configure {
    export type AsObject = {
      packageName: string,
      secretList: Array<SecretDevMap.SecretSpec.AsObject>,
    }
  }

  export class SecretSpec extends jspb.Message {
    getName(): string;
    setName(value: string): void;

    getFromPath(): string;
    setFromPath(value: string): void;

    getValue(): string;
    setValue(value: string): void;

    getResourceName(): string;
    setResourceName(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): SecretSpec.AsObject;
    static toObject(includeInstance: boolean, msg: SecretSpec): SecretSpec.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: SecretSpec, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): SecretSpec;
    static deserializeBinaryFromReader(message: SecretSpec, reader: jspb.BinaryReader): SecretSpec;
  }

  export namespace SecretSpec {
    export type AsObject = {
      name: string,
      fromPath: string,
      value: string,
      resourceName: string,
    }
  }
}

export class Value extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPath(): string;
  setPath(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Value.AsObject;
  static toObject(includeInstance: boolean, msg: Value): Value.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Value, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Value;
  static deserializeBinaryFromReader(message: Value, reader: jspb.BinaryReader): Value;
}

export namespace Value {
  export type AsObject = {
    name: string,
    path: string,
  }
}

