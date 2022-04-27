// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: runtime/docker/config.proto

package docker

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

type Configuration struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Host      string `protobuf:"bytes,1,opt,name=host,proto3" json:"host,omitempty"`
	Version   string `protobuf:"bytes,2,opt,name=version,proto3" json:"version,omitempty"`
	CertPath  string `protobuf:"bytes,3,opt,name=cert_path,json=certPath,proto3" json:"cert_path,omitempty"`
	VerifyTls bool   `protobuf:"varint,4,opt,name=verify_tls,json=verifyTls,proto3" json:"verify_tls,omitempty"`
}

func (x *Configuration) Reset() {
	*x = Configuration{}
	if protoimpl.UnsafeEnabled {
		mi := &file_runtime_docker_config_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Configuration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Configuration) ProtoMessage() {}

func (x *Configuration) ProtoReflect() protoreflect.Message {
	mi := &file_runtime_docker_config_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Configuration.ProtoReflect.Descriptor instead.
func (*Configuration) Descriptor() ([]byte, []int) {
	return file_runtime_docker_config_proto_rawDescGZIP(), []int{0}
}

func (x *Configuration) GetHost() string {
	if x != nil {
		return x.Host
	}
	return ""
}

func (x *Configuration) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *Configuration) GetCertPath() string {
	if x != nil {
		return x.CertPath
	}
	return ""
}

func (x *Configuration) GetVerifyTls() bool {
	if x != nil {
		return x.VerifyTls
	}
	return false
}

var File_runtime_docker_config_proto protoreflect.FileDescriptor

var file_runtime_docker_config_proto_rawDesc = []byte{
	0x0a, 0x1b, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2f, 0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72,
	0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0e, 0x72,
	0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2e, 0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72, 0x22, 0x79, 0x0a,
	0x0d, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12,
	0x0a, 0x04, 0x68, 0x6f, 0x73, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x68, 0x6f,
	0x73, 0x74, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1b, 0x0a, 0x09,
	0x63, 0x65, 0x72, 0x74, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x08, 0x63, 0x65, 0x72, 0x74, 0x50, 0x61, 0x74, 0x68, 0x12, 0x1d, 0x0a, 0x0a, 0x76, 0x65, 0x72,
	0x69, 0x66, 0x79, 0x5f, 0x74, 0x6c, 0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x76,
	0x65, 0x72, 0x69, 0x66, 0x79, 0x54, 0x6c, 0x73, 0x42, 0x2d, 0x5a, 0x2b, 0x6e, 0x61, 0x6d, 0x65,
	0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f,
	0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65,
	0x2f, 0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_runtime_docker_config_proto_rawDescOnce sync.Once
	file_runtime_docker_config_proto_rawDescData = file_runtime_docker_config_proto_rawDesc
)

func file_runtime_docker_config_proto_rawDescGZIP() []byte {
	file_runtime_docker_config_proto_rawDescOnce.Do(func() {
		file_runtime_docker_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_runtime_docker_config_proto_rawDescData)
	})
	return file_runtime_docker_config_proto_rawDescData
}

var file_runtime_docker_config_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_runtime_docker_config_proto_goTypes = []interface{}{
	(*Configuration)(nil), // 0: runtime.docker.Configuration
}
var file_runtime_docker_config_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_runtime_docker_config_proto_init() }
func file_runtime_docker_config_proto_init() {
	if File_runtime_docker_config_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_runtime_docker_config_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Configuration); i {
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
			RawDescriptor: file_runtime_docker_config_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_runtime_docker_config_proto_goTypes,
		DependencyIndexes: file_runtime_docker_config_proto_depIdxs,
		MessageInfos:      file_runtime_docker_config_proto_msgTypes,
	}.Build()
	File_runtime_docker_config_proto = out.File
	file_runtime_docker_config_proto_rawDesc = nil
	file_runtime_docker_config_proto_goTypes = nil
	file_runtime_docker_config_proto_depIdxs = nil
}
