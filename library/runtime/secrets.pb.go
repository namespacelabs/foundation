// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: library/runtime/secrets.proto

package runtime

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type SecretInstance struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path string `protobuf:"bytes,1,opt,name=path,proto3" json:"path,omitempty"`
}

func (x *SecretInstance) Reset() {
	*x = SecretInstance{}
	if protoimpl.UnsafeEnabled {
		mi := &file_library_runtime_secrets_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SecretInstance) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SecretInstance) ProtoMessage() {}

func (x *SecretInstance) ProtoReflect() protoreflect.Message {
	mi := &file_library_runtime_secrets_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SecretInstance.ProtoReflect.Descriptor instead.
func (*SecretInstance) Descriptor() ([]byte, []int) {
	return file_library_runtime_secrets_proto_rawDescGZIP(), []int{0}
}

func (x *SecretInstance) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

var File_library_runtime_secrets_proto protoreflect.FileDescriptor

var file_library_runtime_secrets_proto_rawDesc = []byte{
	0x0a, 0x1d, 0x6c, 0x69, 0x62, 0x72, 0x61, 0x72, 0x79, 0x2f, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d,
	0x65, 0x2f, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12,
	0x1a, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x6c, 0x69, 0x62, 0x72,
	0x61, 0x72, 0x79, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x22, 0x24, 0x0a, 0x0e, 0x53,
	0x65, 0x63, 0x72, 0x65, 0x74, 0x49, 0x6e, 0x73, 0x74, 0x61, 0x6e, 0x63, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74,
	0x68, 0x42, 0x2e, 0x5a, 0x2c, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61,
	0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2f, 0x6c, 0x69, 0x62, 0x72, 0x61, 0x72, 0x79, 0x2f, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d,
	0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_library_runtime_secrets_proto_rawDescOnce sync.Once
	file_library_runtime_secrets_proto_rawDescData = file_library_runtime_secrets_proto_rawDesc
)

func file_library_runtime_secrets_proto_rawDescGZIP() []byte {
	file_library_runtime_secrets_proto_rawDescOnce.Do(func() {
		file_library_runtime_secrets_proto_rawDescData = protoimpl.X.CompressGZIP(file_library_runtime_secrets_proto_rawDescData)
	})
	return file_library_runtime_secrets_proto_rawDescData
}

var file_library_runtime_secrets_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_library_runtime_secrets_proto_goTypes = []interface{}{
	(*SecretInstance)(nil), // 0: foundation.library.runtime.SecretInstance
}
var file_library_runtime_secrets_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_library_runtime_secrets_proto_init() }
func file_library_runtime_secrets_proto_init() {
	if File_library_runtime_secrets_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_library_runtime_secrets_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SecretInstance); i {
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
			RawDescriptor: file_library_runtime_secrets_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_library_runtime_secrets_proto_goTypes,
		DependencyIndexes: file_library_runtime_secrets_proto_depIdxs,
		MessageInfos:      file_library_runtime_secrets_proto_msgTypes,
	}.Build()
	File_library_runtime_secrets_proto = out.File
	file_library_runtime_secrets_proto_rawDesc = nil
	file_library_runtime_secrets_proto_goTypes = nil
	file_library_runtime_secrets_proto_depIdxs = nil
}
