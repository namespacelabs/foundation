// source: std/proto/options.proto
/**
 * @fileoverview
 * @enhanceable
 * @suppress {missingRequire} reports error on implicit type usages.
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!
/* eslint-disable */
// @ts-nocheck

var jspb = require('google-protobuf');
var goog = jspb;
var global = Function('return this')();

var google_protobuf_descriptor_pb = require('google-protobuf/google/protobuf/descriptor_pb.js');
goog.object.extend(proto, google_protobuf_descriptor_pb);
goog.exportSymbol('proto.foundation.std.proto.isPackage', null, global);
goog.exportSymbol('proto.foundation.std.proto.isSensitive', null, global);
goog.exportSymbol('proto.foundation.std.proto.provisionOnly', null, global);

/**
 * A tuple of {field number, class constructor} for the extension
 * field named `isPackage`.
 * @type {!jspb.ExtensionFieldInfo<boolean>}
 */
proto.foundation.std.proto.isPackage = new jspb.ExtensionFieldInfo(
    60000,
    {isPackage: 0},
    null,
     /** @type {?function((boolean|undefined),!jspb.Message=): !Object} */ (
         null),
    0);

google_protobuf_descriptor_pb.FieldOptions.extensionsBinary[60000] = new jspb.ExtensionFieldBinaryInfo(
    proto.foundation.std.proto.isPackage,
    jspb.BinaryReader.prototype.readBool,
    jspb.BinaryWriter.prototype.writeBool,
    undefined,
    undefined,
    false);
// This registers the extension field with the extended class, so that
// toObject() will function correctly.
google_protobuf_descriptor_pb.FieldOptions.extensions[60000] = proto.foundation.std.proto.isPackage;


/**
 * A tuple of {field number, class constructor} for the extension
 * field named `isSensitive`.
 * @type {!jspb.ExtensionFieldInfo<boolean>}
 */
proto.foundation.std.proto.isSensitive = new jspb.ExtensionFieldInfo(
    60001,
    {isSensitive: 0},
    null,
     /** @type {?function((boolean|undefined),!jspb.Message=): !Object} */ (
         null),
    0);

google_protobuf_descriptor_pb.FieldOptions.extensionsBinary[60001] = new jspb.ExtensionFieldBinaryInfo(
    proto.foundation.std.proto.isSensitive,
    jspb.BinaryReader.prototype.readBool,
    jspb.BinaryWriter.prototype.writeBool,
    undefined,
    undefined,
    false);
// This registers the extension field with the extended class, so that
// toObject() will function correctly.
google_protobuf_descriptor_pb.FieldOptions.extensions[60001] = proto.foundation.std.proto.isSensitive;


/**
 * A tuple of {field number, class constructor} for the extension
 * field named `provisionOnly`.
 * @type {!jspb.ExtensionFieldInfo<boolean>}
 */
proto.foundation.std.proto.provisionOnly = new jspb.ExtensionFieldInfo(
    60002,
    {provisionOnly: 0},
    null,
     /** @type {?function((boolean|undefined),!jspb.Message=): !Object} */ (
         null),
    0);

google_protobuf_descriptor_pb.FieldOptions.extensionsBinary[60002] = new jspb.ExtensionFieldBinaryInfo(
    proto.foundation.std.proto.provisionOnly,
    jspb.BinaryReader.prototype.readBool,
    jspb.BinaryWriter.prototype.writeBool,
    undefined,
    undefined,
    false);
// This registers the extension field with the extended class, so that
// toObject() will function correctly.
google_protobuf_descriptor_pb.FieldOptions.extensions[60002] = proto.foundation.std.proto.provisionOnly;

goog.object.extend(exports, proto.foundation.std.proto);
