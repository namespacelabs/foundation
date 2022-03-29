// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: runtime/rtypes/integration.proto

package rtypes

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
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

// Map a local directory to a container. Used for development purposes.
type LocalMapping struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Relative to workspace root.
	LocalPath string `protobuf:"bytes,1,opt,name=local_path,json=localPath,proto3" json:"local_path,omitempty"`
	// Absolute path within the host (overrides local_path).
	HostPath string `protobuf:"bytes,2,opt,name=host_path,json=hostPath,proto3" json:"host_path,omitempty"`
	// Must be an absolute path.
	ContainerPath string `protobuf:"bytes,3,opt,name=container_path,json=containerPath,proto3" json:"container_path,omitempty"`
}

func (x *LocalMapping) Reset() {
	*x = LocalMapping{}
	if protoimpl.UnsafeEnabled {
		mi := &file_runtime_rtypes_integration_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LocalMapping) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LocalMapping) ProtoMessage() {}

func (x *LocalMapping) ProtoReflect() protoreflect.Message {
	mi := &file_runtime_rtypes_integration_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LocalMapping.ProtoReflect.Descriptor instead.
func (*LocalMapping) Descriptor() ([]byte, []int) {
	return file_runtime_rtypes_integration_proto_rawDescGZIP(), []int{0}
}

func (x *LocalMapping) GetLocalPath() string {
	if x != nil {
		return x.LocalPath
	}
	return ""
}

func (x *LocalMapping) GetHostPath() string {
	if x != nil {
		return x.HostPath
	}
	return ""
}

func (x *LocalMapping) GetContainerPath() string {
	if x != nil {
		return x.ContainerPath
	}
	return ""
}

type Arg struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name  string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Value string `protobuf:"bytes,2,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Arg) Reset() {
	*x = Arg{}
	if protoimpl.UnsafeEnabled {
		mi := &file_runtime_rtypes_integration_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Arg) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Arg) ProtoMessage() {}

func (x *Arg) ProtoReflect() protoreflect.Message {
	mi := &file_runtime_rtypes_integration_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Arg.ProtoReflect.Descriptor instead.
func (*Arg) Descriptor() ([]byte, []int) {
	return file_runtime_rtypes_integration_proto_rawDescGZIP(), []int{1}
}

func (x *Arg) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Arg) GetValue() string {
	if x != nil {
		return x.Value
	}
	return ""
}

type Port struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name          string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	ContainerPort int32  `protobuf:"varint,2,opt,name=container_port,json=containerPort,proto3" json:"container_port,omitempty"`
	HostPort      int32  `protobuf:"varint,3,opt,name=host_port,json=hostPort,proto3" json:"host_port,omitempty"`
}

func (x *Port) Reset() {
	*x = Port{}
	if protoimpl.UnsafeEnabled {
		mi := &file_runtime_rtypes_integration_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Port) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Port) ProtoMessage() {}

func (x *Port) ProtoReflect() protoreflect.Message {
	mi := &file_runtime_rtypes_integration_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Port.ProtoReflect.Descriptor instead.
func (*Port) Descriptor() ([]byte, []int) {
	return file_runtime_rtypes_integration_proto_rawDescGZIP(), []int{2}
}

func (x *Port) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Port) GetContainerPort() int32 {
	if x != nil {
		return x.ContainerPort
	}
	return 0
}

func (x *Port) GetHostPort() int32 {
	if x != nil {
		return x.HostPort
	}
	return 0
}

type ProvisionProps struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	LocalMapping   []*LocalMapping      `protobuf:"bytes,1,rep,name=local_mapping,json=localMapping,proto3" json:"local_mapping,omitempty"`
	ProvisionInput []*anypb.Any         `protobuf:"bytes,2,rep,name=provision_input,json=provisionInput,proto3" json:"provision_input,omitempty"`
	Definition     []*schema.Definition `protobuf:"bytes,3,rep,name=definition,proto3" json:"definition,omitempty"`
}

func (x *ProvisionProps) Reset() {
	*x = ProvisionProps{}
	if protoimpl.UnsafeEnabled {
		mi := &file_runtime_rtypes_integration_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ProvisionProps) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ProvisionProps) ProtoMessage() {}

func (x *ProvisionProps) ProtoReflect() protoreflect.Message {
	mi := &file_runtime_rtypes_integration_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ProvisionProps.ProtoReflect.Descriptor instead.
func (*ProvisionProps) Descriptor() ([]byte, []int) {
	return file_runtime_rtypes_integration_proto_rawDescGZIP(), []int{3}
}

func (x *ProvisionProps) GetLocalMapping() []*LocalMapping {
	if x != nil {
		return x.LocalMapping
	}
	return nil
}

func (x *ProvisionProps) GetProvisionInput() []*anypb.Any {
	if x != nil {
		return x.ProvisionInput
	}
	return nil
}

func (x *ProvisionProps) GetDefinition() []*schema.Definition {
	if x != nil {
		return x.Definition
	}
	return nil
}

var File_runtime_rtypes_integration_proto protoreflect.FileDescriptor

var file_runtime_rtypes_integration_proto_rawDesc = []byte{
	0x0a, 0x20, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2f, 0x72, 0x74, 0x79, 0x70, 0x65, 0x73,
	0x2f, 0x69, 0x6e, 0x74, 0x65, 0x67, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x19, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x72,
	0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2e, 0x72, 0x74, 0x79, 0x70, 0x65, 0x73, 0x1a, 0x19, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x61,
	0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x17, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x2f, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x71, 0x0a, 0x0c, 0x4c, 0x6f, 0x63, 0x61, 0x6c, 0x4d, 0x61, 0x70, 0x70, 0x69, 0x6e,
	0x67, 0x12, 0x1d, 0x0a, 0x0a, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x50, 0x61, 0x74, 0x68,
	0x12, 0x1b, 0x0a, 0x09, 0x68, 0x6f, 0x73, 0x74, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x68, 0x6f, 0x73, 0x74, 0x50, 0x61, 0x74, 0x68, 0x12, 0x25, 0x0a,
	0x0e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x5f, 0x70, 0x61, 0x74, 0x68, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72,
	0x50, 0x61, 0x74, 0x68, 0x22, 0x2f, 0x0a, 0x03, 0x41, 0x72, 0x67, 0x12, 0x12, 0x0a, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12,
	0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x5e, 0x0a, 0x04, 0x50, 0x6f, 0x72, 0x74, 0x12, 0x12, 0x0a,
	0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d,
	0x65, 0x12, 0x25, 0x0a, 0x0e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x5f, 0x70,
	0x6f, 0x72, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0d, 0x63, 0x6f, 0x6e, 0x74, 0x61,
	0x69, 0x6e, 0x65, 0x72, 0x50, 0x6f, 0x72, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x68, 0x6f, 0x73, 0x74,
	0x5f, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x68, 0x6f, 0x73,
	0x74, 0x50, 0x6f, 0x72, 0x74, 0x22, 0xdc, 0x01, 0x0a, 0x0e, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x73,
	0x69, 0x6f, 0x6e, 0x50, 0x72, 0x6f, 0x70, 0x73, 0x12, 0x4c, 0x0a, 0x0d, 0x6c, 0x6f, 0x63, 0x61,
	0x6c, 0x5f, 0x6d, 0x61, 0x70, 0x70, 0x69, 0x6e, 0x67, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x27, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x72, 0x75, 0x6e,
	0x74, 0x69, 0x6d, 0x65, 0x2e, 0x72, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x4c, 0x6f, 0x63, 0x61,
	0x6c, 0x4d, 0x61, 0x70, 0x70, 0x69, 0x6e, 0x67, 0x52, 0x0c, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x4d,
	0x61, 0x70, 0x70, 0x69, 0x6e, 0x67, 0x12, 0x3d, 0x0a, 0x0f, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x73,
	0x69, 0x6f, 0x6e, 0x5f, 0x69, 0x6e, 0x70, 0x75, 0x74, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75,
	0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52, 0x0e, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x73, 0x69, 0x6f, 0x6e,
	0x49, 0x6e, 0x70, 0x75, 0x74, 0x12, 0x3d, 0x0a, 0x0a, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74,
	0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x66, 0x6f, 0x75, 0x6e,
	0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x44, 0x65,
	0x66, 0x69, 0x6e, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0a, 0x64, 0x65, 0x66, 0x69, 0x6e, 0x69,
	0x74, 0x69, 0x6f, 0x6e, 0x42, 0x2d, 0x5a, 0x2b, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2f, 0x72, 0x74, 0x79,
	0x70, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_runtime_rtypes_integration_proto_rawDescOnce sync.Once
	file_runtime_rtypes_integration_proto_rawDescData = file_runtime_rtypes_integration_proto_rawDesc
)

func file_runtime_rtypes_integration_proto_rawDescGZIP() []byte {
	file_runtime_rtypes_integration_proto_rawDescOnce.Do(func() {
		file_runtime_rtypes_integration_proto_rawDescData = protoimpl.X.CompressGZIP(file_runtime_rtypes_integration_proto_rawDescData)
	})
	return file_runtime_rtypes_integration_proto_rawDescData
}

var file_runtime_rtypes_integration_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_runtime_rtypes_integration_proto_goTypes = []interface{}{
	(*LocalMapping)(nil),      // 0: foundation.runtime.rtypes.LocalMapping
	(*Arg)(nil),               // 1: foundation.runtime.rtypes.Arg
	(*Port)(nil),              // 2: foundation.runtime.rtypes.Port
	(*ProvisionProps)(nil),    // 3: foundation.runtime.rtypes.ProvisionProps
	(*anypb.Any)(nil),         // 4: google.protobuf.Any
	(*schema.Definition)(nil), // 5: foundation.schema.Definition
}
var file_runtime_rtypes_integration_proto_depIdxs = []int32{
	0, // 0: foundation.runtime.rtypes.ProvisionProps.local_mapping:type_name -> foundation.runtime.rtypes.LocalMapping
	4, // 1: foundation.runtime.rtypes.ProvisionProps.provision_input:type_name -> google.protobuf.Any
	5, // 2: foundation.runtime.rtypes.ProvisionProps.definition:type_name -> foundation.schema.Definition
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_runtime_rtypes_integration_proto_init() }
func file_runtime_rtypes_integration_proto_init() {
	if File_runtime_rtypes_integration_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_runtime_rtypes_integration_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LocalMapping); i {
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
		file_runtime_rtypes_integration_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Arg); i {
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
		file_runtime_rtypes_integration_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Port); i {
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
		file_runtime_rtypes_integration_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ProvisionProps); i {
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
			RawDescriptor: file_runtime_rtypes_integration_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_runtime_rtypes_integration_proto_goTypes,
		DependencyIndexes: file_runtime_rtypes_integration_proto_depIdxs,
		MessageInfos:      file_runtime_rtypes_integration_proto_msgTypes,
	}.Build()
	File_runtime_rtypes_integration_proto = out.File
	file_runtime_rtypes_integration_proto_rawDesc = nil
	file_runtime_rtypes_integration_proto_goTypes = nil
	file_runtime_rtypes_integration_proto_depIdxs = nil
}