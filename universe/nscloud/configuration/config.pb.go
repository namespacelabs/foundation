// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: universe/nscloud/configuration/config.proto

package configuration

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

// Used as a configuration provider in an environment.
type Cluster struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ClusterId       string `protobuf:"bytes,1,opt,name=cluster_id,json=clusterId,proto3" json:"cluster_id,omitempty"`
	ApiEndpoint     string `protobuf:"bytes,3,opt,name=api_endpoint,json=apiEndpoint,proto3" json:"api_endpoint,omitempty"`
	UseBuildCluster bool   `protobuf:"varint,2,opt,name=use_build_cluster,json=useBuildCluster,proto3" json:"use_build_cluster,omitempty"`
}

func (x *Cluster) Reset() {
	*x = Cluster{}
	if protoimpl.UnsafeEnabled {
		mi := &file_universe_nscloud_configuration_config_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Cluster) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Cluster) ProtoMessage() {}

func (x *Cluster) ProtoReflect() protoreflect.Message {
	mi := &file_universe_nscloud_configuration_config_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Cluster.ProtoReflect.Descriptor instead.
func (*Cluster) Descriptor() ([]byte, []int) {
	return file_universe_nscloud_configuration_config_proto_rawDescGZIP(), []int{0}
}

func (x *Cluster) GetClusterId() string {
	if x != nil {
		return x.ClusterId
	}
	return ""
}

func (x *Cluster) GetApiEndpoint() string {
	if x != nil {
		return x.ApiEndpoint
	}
	return ""
}

func (x *Cluster) GetUseBuildCluster() bool {
	if x != nil {
		return x.UseBuildCluster
	}
	return false
}

var File_universe_nscloud_configuration_config_proto protoreflect.FileDescriptor

var file_universe_nscloud_configuration_config_proto_rawDesc = []byte{
	0x0a, 0x2b, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2f, 0x6e, 0x73, 0x63, 0x6c, 0x6f,
	0x75, 0x64, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x29, 0x66,
	0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72,
	0x73, 0x65, 0x2e, 0x6e, 0x73, 0x63, 0x6c, 0x6f, 0x75, 0x64, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x77, 0x0a, 0x07, 0x43, 0x6c, 0x75, 0x73,
	0x74, 0x65, 0x72, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x5f, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72,
	0x49, 0x64, 0x12, 0x21, 0x0a, 0x0c, 0x61, 0x70, 0x69, 0x5f, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69,
	0x6e, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x61, 0x70, 0x69, 0x45, 0x6e, 0x64,
	0x70, 0x6f, 0x69, 0x6e, 0x74, 0x12, 0x2a, 0x0a, 0x11, 0x75, 0x73, 0x65, 0x5f, 0x62, 0x75, 0x69,
	0x6c, 0x64, 0x5f, 0x63, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x0f, 0x75, 0x73, 0x65, 0x42, 0x75, 0x69, 0x6c, 0x64, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65,
	0x72, 0x42, 0x3d, 0x5a, 0x3b, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61,
	0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2f, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2f, 0x6e, 0x73, 0x63, 0x6c, 0x6f,
	0x75, 0x64, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_universe_nscloud_configuration_config_proto_rawDescOnce sync.Once
	file_universe_nscloud_configuration_config_proto_rawDescData = file_universe_nscloud_configuration_config_proto_rawDesc
)

func file_universe_nscloud_configuration_config_proto_rawDescGZIP() []byte {
	file_universe_nscloud_configuration_config_proto_rawDescOnce.Do(func() {
		file_universe_nscloud_configuration_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_universe_nscloud_configuration_config_proto_rawDescData)
	})
	return file_universe_nscloud_configuration_config_proto_rawDescData
}

var file_universe_nscloud_configuration_config_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_universe_nscloud_configuration_config_proto_goTypes = []interface{}{
	(*Cluster)(nil), // 0: foundation.universe.nscloud.configuration.Cluster
}
var file_universe_nscloud_configuration_config_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_universe_nscloud_configuration_config_proto_init() }
func file_universe_nscloud_configuration_config_proto_init() {
	if File_universe_nscloud_configuration_config_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_universe_nscloud_configuration_config_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Cluster); i {
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
			RawDescriptor: file_universe_nscloud_configuration_config_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_universe_nscloud_configuration_config_proto_goTypes,
		DependencyIndexes: file_universe_nscloud_configuration_config_proto_depIdxs,
		MessageInfos:      file_universe_nscloud_configuration_config_proto_msgTypes,
	}.Build()
	File_universe_nscloud_configuration_config_proto = out.File
	file_universe_nscloud_configuration_config_proto_rawDesc = nil
	file_universe_nscloud_configuration_config_proto_goTypes = nil
	file_universe_nscloud_configuration_config_proto_depIdxs = nil
}
