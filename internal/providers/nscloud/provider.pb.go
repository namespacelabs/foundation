// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: internal/providers/nscloud/provider.proto

package nscloud

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

type PrebuiltCluster struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ClusterId        string `protobuf:"bytes,1,opt,name=cluster_id,json=clusterId,proto3" json:"cluster_id,omitempty"`
	ApiEndpoint      string `protobuf:"bytes,4,opt,name=api_endpoint,json=apiEndpoint,proto3" json:"api_endpoint,omitempty"`
	SerializedConfig []byte `protobuf:"bytes,2,opt,name=serialized_config,json=serializedConfig,proto3" json:"serialized_config,omitempty"` // Deprecated, always fetched now.
	Ephemeral        bool   `protobuf:"varint,3,opt,name=ephemeral,proto3" json:"ephemeral,omitempty"`
}

func (x *PrebuiltCluster) Reset() {
	*x = PrebuiltCluster{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_providers_nscloud_provider_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PrebuiltCluster) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PrebuiltCluster) ProtoMessage() {}

func (x *PrebuiltCluster) ProtoReflect() protoreflect.Message {
	mi := &file_internal_providers_nscloud_provider_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PrebuiltCluster.ProtoReflect.Descriptor instead.
func (*PrebuiltCluster) Descriptor() ([]byte, []int) {
	return file_internal_providers_nscloud_provider_proto_rawDescGZIP(), []int{0}
}

func (x *PrebuiltCluster) GetClusterId() string {
	if x != nil {
		return x.ClusterId
	}
	return ""
}

func (x *PrebuiltCluster) GetApiEndpoint() string {
	if x != nil {
		return x.ApiEndpoint
	}
	return ""
}

func (x *PrebuiltCluster) GetSerializedConfig() []byte {
	if x != nil {
		return x.SerializedConfig
	}
	return nil
}

func (x *PrebuiltCluster) GetEphemeral() bool {
	if x != nil {
		return x.Ephemeral
	}
	return false
}

var File_internal_providers_nscloud_provider_proto protoreflect.FileDescriptor

var file_internal_providers_nscloud_provider_proto_rawDesc = []byte{
	0x0a, 0x29, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x76, 0x69,
	0x64, 0x65, 0x72, 0x73, 0x2f, 0x6e, 0x73, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2f, 0x70, 0x72, 0x6f,
	0x76, 0x69, 0x64, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1c, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72,
	0x73, 0x2e, 0x6e, 0x73, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x22, 0x9e, 0x01, 0x0a, 0x0f, 0x50, 0x72,
	0x65, 0x62, 0x75, 0x69, 0x6c, 0x74, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0x1d, 0x0a,
	0x0a, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x09, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x49, 0x64, 0x12, 0x21, 0x0a, 0x0c,
	0x61, 0x70, 0x69, 0x5f, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x18, 0x04, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x0b, 0x61, 0x70, 0x69, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x12,
	0x2b, 0x0a, 0x11, 0x73, 0x65, 0x72, 0x69, 0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x5f, 0x63, 0x6f,
	0x6e, 0x66, 0x69, 0x67, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x10, 0x73, 0x65, 0x72, 0x69,
	0x61, 0x6c, 0x69, 0x7a, 0x65, 0x64, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x1c, 0x0a, 0x09,
	0x65, 0x70, 0x68, 0x65, 0x6d, 0x65, 0x72, 0x61, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x09, 0x65, 0x70, 0x68, 0x65, 0x6d, 0x65, 0x72, 0x61, 0x6c, 0x42, 0x39, 0x5a, 0x37, 0x6e, 0x61,
	0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f,
	0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72,
	0x6e, 0x61, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x73, 0x2f, 0x6e, 0x73,
	0x63, 0x6c, 0x6f, 0x75, 0x64, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_internal_providers_nscloud_provider_proto_rawDescOnce sync.Once
	file_internal_providers_nscloud_provider_proto_rawDescData = file_internal_providers_nscloud_provider_proto_rawDesc
)

func file_internal_providers_nscloud_provider_proto_rawDescGZIP() []byte {
	file_internal_providers_nscloud_provider_proto_rawDescOnce.Do(func() {
		file_internal_providers_nscloud_provider_proto_rawDescData = protoimpl.X.CompressGZIP(file_internal_providers_nscloud_provider_proto_rawDescData)
	})
	return file_internal_providers_nscloud_provider_proto_rawDescData
}

var file_internal_providers_nscloud_provider_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_internal_providers_nscloud_provider_proto_goTypes = []interface{}{
	(*PrebuiltCluster)(nil), // 0: foundation.providers.nscloud.PrebuiltCluster
}
var file_internal_providers_nscloud_provider_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_internal_providers_nscloud_provider_proto_init() }
func file_internal_providers_nscloud_provider_proto_init() {
	if File_internal_providers_nscloud_provider_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_internal_providers_nscloud_provider_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PrebuiltCluster); i {
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
			RawDescriptor: file_internal_providers_nscloud_provider_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_internal_providers_nscloud_provider_proto_goTypes,
		DependencyIndexes: file_internal_providers_nscloud_provider_proto_depIdxs,
		MessageInfos:      file_internal_providers_nscloud_provider_proto_msgTypes,
	}.Build()
	File_internal_providers_nscloud_provider_proto = out.File
	file_internal_providers_nscloud_provider_proto_rawDesc = nil
	file_internal_providers_nscloud_provider_proto_goTypes = nil
	file_internal_providers_nscloud_provider_proto_depIdxs = nil
}
