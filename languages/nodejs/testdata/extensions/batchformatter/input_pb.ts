// @generated by protobuf-ts 2.7.0 with parameter force_disable_services,add_pb_suffix
// @generated from protobuf file "languages/nodejs/testdata/extensions/batchformatter/input.proto" (package "languages.nodejs.testdata.extensions.batchformatter", syntax proto3)
// tslint:disable
//
// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation
//
import type { BinaryWriteOptions } from "@protobuf-ts/runtime";
import type { IBinaryWriter } from "@protobuf-ts/runtime";
import { UnknownFieldHandler } from "@protobuf-ts/runtime";
import type { BinaryReadOptions } from "@protobuf-ts/runtime";
import type { IBinaryReader } from "@protobuf-ts/runtime";
import type { PartialMessage } from "@protobuf-ts/runtime";
import { reflectionMergePartial } from "@protobuf-ts/runtime";
import { MESSAGE_TYPE } from "@protobuf-ts/runtime";
import { MessageType } from "@protobuf-ts/runtime";
/**
 * @generated from protobuf message languages.nodejs.testdata.extensions.batchformatter.InputData
 */
export interface InputData {
}
// @generated message type with reflection information, may provide speed optimized methods
class InputData$Type extends MessageType<InputData> {
    constructor() {
        super("languages.nodejs.testdata.extensions.batchformatter.InputData", []);
    }
    create(value?: PartialMessage<InputData>): InputData {
        const message = {};
        globalThis.Object.defineProperty(message, MESSAGE_TYPE, { enumerable: false, value: this });
        if (value !== undefined)
            reflectionMergePartial<InputData>(this, message, value);
        return message;
    }
    internalBinaryRead(reader: IBinaryReader, length: number, options: BinaryReadOptions, target?: InputData): InputData {
        return target ?? this.create();
    }
    internalBinaryWrite(message: InputData, writer: IBinaryWriter, options: BinaryWriteOptions): IBinaryWriter {
        let u = options.writeUnknownFields;
        if (u !== false)
            (u == true ? UnknownFieldHandler.onWrite : u)(this.typeName, message, writer);
        return writer;
    }
}
/**
 * @generated MessageType for protobuf message languages.nodejs.testdata.extensions.batchformatter.InputData
 */
export const InputData = new InputData$Type();
