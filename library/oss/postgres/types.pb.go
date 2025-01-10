// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: library/oss/postgres/types.proto

package postgres

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	schema "namespacelabs.dev/foundation/schema"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ClusterIntent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// If set, overrides the server package used to instantiate the local database cluster.
	Server *schema.PackageRef `protobuf:"bytes,1,opt,name=server,proto3" json:"server,omitempty"`
	// If set, overrides the root password used to access the cluster.
	PasswordSecret *schema.PackageRef `protobuf:"bytes,2,opt,name=password_secret,json=passwordSecret,proto3" json:"password_secret,omitempty"`
}

func (x *ClusterIntent) Reset() {
	*x = ClusterIntent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_library_oss_postgres_types_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ClusterIntent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ClusterIntent) ProtoMessage() {}

func (x *ClusterIntent) ProtoReflect() protoreflect.Message {
	mi := &file_library_oss_postgres_types_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ClusterIntent.ProtoReflect.Descriptor instead.
func (*ClusterIntent) Descriptor() ([]byte, []int) {
	return file_library_oss_postgres_types_proto_rawDescGZIP(), []int{0}
}

func (x *ClusterIntent) GetServer() *schema.PackageRef {
	if x != nil {
		return x.Server
	}
	return nil
}

func (x *ClusterIntent) GetPasswordSecret() *schema.PackageRef {
	if x != nil {
		return x.PasswordSecret
	}
	return nil
}

type DatabaseIntent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The database name is applied as is (e.g. it is case-sensitive).
	Name                             string                 `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Schema                           []*schema.FileContents `protobuf:"bytes,2,rep,name=schema,proto3" json:"schema,omitempty"`
	SkipSchemaInitializationIfExists bool                   `protobuf:"varint,3,opt,name=skip_schema_initialization_if_exists,json=skipSchemaInitializationIfExists,proto3" json:"skip_schema_initialization_if_exists,omitempty"`
	ProvisionHelperFunctions         bool                   `protobuf:"varint,4,opt,name=provision_helper_functions,json=provisionHelperFunctions,proto3" json:"provision_helper_functions,omitempty"`
}

func (x *DatabaseIntent) Reset() {
	*x = DatabaseIntent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_library_oss_postgres_types_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DatabaseIntent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DatabaseIntent) ProtoMessage() {}

func (x *DatabaseIntent) ProtoReflect() protoreflect.Message {
	mi := &file_library_oss_postgres_types_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DatabaseIntent.ProtoReflect.Descriptor instead.
func (*DatabaseIntent) Descriptor() ([]byte, []int) {
	return file_library_oss_postgres_types_proto_rawDescGZIP(), []int{1}
}

func (x *DatabaseIntent) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *DatabaseIntent) GetSchema() []*schema.FileContents {
	if x != nil {
		return x.Schema
	}
	return nil
}

func (x *DatabaseIntent) GetSkipSchemaInitializationIfExists() bool {
	if x != nil {
		return x.SkipSchemaInitializationIfExists
	}
	return false
}

func (x *DatabaseIntent) GetProvisionHelperFunctions() bool {
	if x != nil {
		return x.ProvisionHelperFunctions
	}
	return false
}

var File_library_oss_postgres_types_proto protoreflect.FileDescriptor

var file_library_oss_postgres_types_proto_rawDesc = []byte{
	0x0a, 0x20, 0x6c, 0x69, 0x62, 0x72, 0x61, 0x72, 0x79, 0x2f, 0x6f, 0x73, 0x73, 0x2f, 0x70, 0x6f,
	0x73, 0x74, 0x67, 0x72, 0x65, 0x73, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x14, 0x6c, 0x69, 0x62, 0x72, 0x61, 0x72, 0x79, 0x2e, 0x6f, 0x73, 0x73, 0x2e,
	0x70, 0x6f, 0x73, 0x74, 0x67, 0x72, 0x65, 0x73, 0x1a, 0x14, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x2f, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x19,
	0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x66, 0x69, 0x6c, 0x65, 0x63, 0x6f, 0x6e, 0x74, 0x65,
	0x6e, 0x74, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x8e, 0x01, 0x0a, 0x0d, 0x43, 0x6c,
	0x75, 0x73, 0x74, 0x65, 0x72, 0x49, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x12, 0x35, 0x0a, 0x06, 0x73,
	0x65, 0x72, 0x76, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x66, 0x6f,
	0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e,
	0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x52, 0x65, 0x66, 0x52, 0x06, 0x73, 0x65, 0x72, 0x76,
	0x65, 0x72, 0x12, 0x46, 0x0a, 0x0f, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x5f, 0x73,
	0x65, 0x63, 0x72, 0x65, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x66, 0x6f,
	0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e,
	0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x52, 0x65, 0x66, 0x52, 0x0e, 0x70, 0x61, 0x73, 0x73,
	0x77, 0x6f, 0x72, 0x64, 0x53, 0x65, 0x63, 0x72, 0x65, 0x74, 0x22, 0xeb, 0x01, 0x0a, 0x0e, 0x44,
	0x61, 0x74, 0x61, 0x62, 0x61, 0x73, 0x65, 0x49, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x12, 0x12, 0x0a,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x12, 0x37, 0x0a, 0x06, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x1f, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73,
	0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x46, 0x69, 0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e,
	0x74, 0x73, 0x52, 0x06, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x12, 0x4e, 0x0a, 0x24, 0x73, 0x6b,
	0x69, 0x70, 0x5f, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x5f, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x61,
	0x6c, 0x69, 0x7a, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x66, 0x5f, 0x65, 0x78, 0x69, 0x73,
	0x74, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x20, 0x73, 0x6b, 0x69, 0x70, 0x53, 0x63,
	0x68, 0x65, 0x6d, 0x61, 0x49, 0x6e, 0x69, 0x74, 0x69, 0x61, 0x6c, 0x69, 0x7a, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x49, 0x66, 0x45, 0x78, 0x69, 0x73, 0x74, 0x73, 0x12, 0x3c, 0x0a, 0x1a, 0x70, 0x72,
	0x6f, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x68, 0x65, 0x6c, 0x70, 0x65, 0x72, 0x5f, 0x66,
	0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x18,
	0x70, 0x72, 0x6f, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e, 0x48, 0x65, 0x6c, 0x70, 0x65, 0x72, 0x46,
	0x75, 0x6e, 0x63, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x42, 0x33, 0x5a, 0x31, 0x6e, 0x61, 0x6d, 0x65,
	0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f,
	0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x6c, 0x69, 0x62, 0x72, 0x61, 0x72, 0x79,
	0x2f, 0x6f, 0x73, 0x73, 0x2f, 0x70, 0x6f, 0x73, 0x74, 0x67, 0x72, 0x65, 0x73, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_library_oss_postgres_types_proto_rawDescOnce sync.Once
	file_library_oss_postgres_types_proto_rawDescData = file_library_oss_postgres_types_proto_rawDesc
)

func file_library_oss_postgres_types_proto_rawDescGZIP() []byte {
	file_library_oss_postgres_types_proto_rawDescOnce.Do(func() {
		file_library_oss_postgres_types_proto_rawDescData = protoimpl.X.CompressGZIP(file_library_oss_postgres_types_proto_rawDescData)
	})
	return file_library_oss_postgres_types_proto_rawDescData
}

var file_library_oss_postgres_types_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_library_oss_postgres_types_proto_goTypes = []interface{}{
	(*ClusterIntent)(nil),       // 0: library.oss.postgres.ClusterIntent
	(*DatabaseIntent)(nil),      // 1: library.oss.postgres.DatabaseIntent
	(*schema.PackageRef)(nil),   // 2: foundation.schema.PackageRef
	(*schema.FileContents)(nil), // 3: foundation.schema.FileContents
}
var file_library_oss_postgres_types_proto_depIdxs = []int32{
	2, // 0: library.oss.postgres.ClusterIntent.server:type_name -> foundation.schema.PackageRef
	2, // 1: library.oss.postgres.ClusterIntent.password_secret:type_name -> foundation.schema.PackageRef
	3, // 2: library.oss.postgres.DatabaseIntent.schema:type_name -> foundation.schema.FileContents
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_library_oss_postgres_types_proto_init() }
func file_library_oss_postgres_types_proto_init() {
	if File_library_oss_postgres_types_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_library_oss_postgres_types_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ClusterIntent); i {
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
		file_library_oss_postgres_types_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DatabaseIntent); i {
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
			RawDescriptor: file_library_oss_postgres_types_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_library_oss_postgres_types_proto_goTypes,
		DependencyIndexes: file_library_oss_postgres_types_proto_depIdxs,
		MessageInfos:      file_library_oss_postgres_types_proto_msgTypes,
	}.Build()
	File_library_oss_postgres_types_proto = out.File
	file_library_oss_postgres_types_proto_rawDesc = nil
	file_library_oss_postgres_types_proto_goTypes = nil
	file_library_oss_postgres_types_proto_depIdxs = nil
}
