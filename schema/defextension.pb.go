// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: schema/defextension.proto

package schema

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type DefExtension struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Description string     `protobuf:"bytes,1,opt,name=description,proto3" json:"description,omitempty"`
	Impl        *anypb.Any `protobuf:"bytes,3,opt,name=impl,proto3" json:"impl,omitempty"`
}

func (x *DefExtension) Reset() {
	*x = DefExtension{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_defextension_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DefExtension) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DefExtension) ProtoMessage() {}

func (x *DefExtension) ProtoReflect() protoreflect.Message {
	mi := &file_schema_defextension_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DefExtension.ProtoReflect.Descriptor instead.
func (*DefExtension) Descriptor() ([]byte, []int) {
	return file_schema_defextension_proto_rawDescGZIP(), []int{0}
}

func (x *DefExtension) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *DefExtension) GetImpl() *anypb.Any {
	if x != nil {
		return x.Impl
	}
	return nil
}

type SerializedMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name  []string `protobuf:"bytes,1,rep,name=name,proto3" json:"name,omitempty"` // Full proto name.
	Value []byte   `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *SerializedMessage) Reset() {
	*x = SerializedMessage{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_defextension_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SerializedMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SerializedMessage) ProtoMessage() {}

func (x *SerializedMessage) ProtoReflect() protoreflect.Message {
	mi := &file_schema_defextension_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SerializedMessage.ProtoReflect.Descriptor instead.
func (*SerializedMessage) Descriptor() ([]byte, []int) {
	return file_schema_defextension_proto_rawDescGZIP(), []int{1}
}

func (x *SerializedMessage) GetName() []string {
	if x != nil {
		return x.Name
	}
	return nil
}

func (x *SerializedMessage) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

var File_schema_defextension_proto protoreflect.FileDescriptor

var file_schema_defextension_proto_rawDesc = []byte{
	0x0a, 0x19, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x64, 0x65, 0x66, 0x65, 0x78, 0x74, 0x65,
	0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x11, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x1a, 0x19,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x60, 0x0a, 0x0c, 0x44, 0x65, 0x66,
	0x45, 0x78, 0x74, 0x65, 0x6e, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73,
	0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b,
	0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x28, 0x0a, 0x04, 0x69,
	0x6d, 0x70, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52,
	0x04, 0x69, 0x6d, 0x70, 0x6c, 0x4a, 0x04, 0x08, 0x02, 0x10, 0x03, 0x22, 0x3d, 0x0a, 0x11, 0x53,
	0x65, 0x72, 0x69, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x0c, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x25, 0x5a, 0x23, 0x6e, 0x61,
	0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f,
	0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x6d,
	0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_schema_defextension_proto_rawDescOnce sync.Once
	file_schema_defextension_proto_rawDescData = file_schema_defextension_proto_rawDesc
)

func file_schema_defextension_proto_rawDescGZIP() []byte {
	file_schema_defextension_proto_rawDescOnce.Do(func() {
		file_schema_defextension_proto_rawDescData = protoimpl.X.CompressGZIP(file_schema_defextension_proto_rawDescData)
	})
	return file_schema_defextension_proto_rawDescData
}

var file_schema_defextension_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_schema_defextension_proto_goTypes = []interface{}{
	(*DefExtension)(nil),      // 0: foundation.schema.DefExtension
	(*SerializedMessage)(nil), // 1: foundation.schema.SerializedMessage
	(*anypb.Any)(nil),         // 2: google.protobuf.Any
}
var file_schema_defextension_proto_depIdxs = []int32{
	2, // 0: foundation.schema.DefExtension.impl:type_name -> google.protobuf.Any
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_schema_defextension_proto_init() }
func file_schema_defextension_proto_init() {
	if File_schema_defextension_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_schema_defextension_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DefExtension); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_schema_defextension_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SerializedMessage); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_schema_defextension_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_schema_defextension_proto_goTypes,
		DependencyIndexes: file_schema_defextension_proto_depIdxs,
		MessageInfos:      file_schema_defextension_proto_msgTypes,
	}.Build()
	File_schema_defextension_proto = out.File
	file_schema_defextension_proto_rawDesc = nil
	file_schema_defextension_proto_goTypes = nil
	file_schema_defextension_proto_depIdxs = nil
}
