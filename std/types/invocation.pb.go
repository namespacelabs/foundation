// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: std/types/invocation.proto

package types

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
	schema "namespacelabs.dev/foundation/schema"
	_ "namespacelabs.dev/foundation/std/proto"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type DeferredInvocationSource struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Binary    string     `protobuf:"bytes,1,opt,name=binary,proto3" json:"binary,omitempty"`
	Cacheable bool       `protobuf:"varint,2,opt,name=cacheable,proto3" json:"cacheable,omitempty"`
	WithInput *anypb.Any `protobuf:"bytes,3,opt,name=with_input,json=withInput,proto3" json:"with_input,omitempty"`
}

func (x *DeferredInvocationSource) Reset() {
	*x = DeferredInvocationSource{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_types_invocation_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeferredInvocationSource) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeferredInvocationSource) ProtoMessage() {}

func (x *DeferredInvocationSource) ProtoReflect() protoreflect.Message {
	mi := &file_std_types_invocation_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeferredInvocationSource.ProtoReflect.Descriptor instead.
func (*DeferredInvocationSource) Descriptor() ([]byte, []int) {
	return file_std_types_invocation_proto_rawDescGZIP(), []int{0}
}

func (x *DeferredInvocationSource) GetBinary() string {
	if x != nil {
		return x.Binary
	}
	return ""
}

func (x *DeferredInvocationSource) GetCacheable() bool {
	if x != nil {
		return x.Cacheable
	}
	return false
}

func (x *DeferredInvocationSource) GetWithInput() *anypb.Any {
	if x != nil {
		return x.WithInput
	}
	return nil
}

type DeferredInvocation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BinaryPackage string               `protobuf:"bytes,1,opt,name=binary_package,json=binaryPackage,proto3" json:"binary_package,omitempty"`
	Image         string               `protobuf:"bytes,2,opt,name=image,proto3" json:"image,omitempty"`
	BinaryConfig  *schema.BinaryConfig `protobuf:"bytes,3,opt,name=binary_config,json=binaryConfig,proto3" json:"binary_config,omitempty"`
	Cacheable     bool                 `protobuf:"varint,4,opt,name=cacheable,proto3" json:"cacheable,omitempty"`
	WithInput     *anypb.Any           `protobuf:"bytes,5,opt,name=with_input,json=withInput,proto3" json:"with_input,omitempty"`
}

func (x *DeferredInvocation) Reset() {
	*x = DeferredInvocation{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_types_invocation_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeferredInvocation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeferredInvocation) ProtoMessage() {}

func (x *DeferredInvocation) ProtoReflect() protoreflect.Message {
	mi := &file_std_types_invocation_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeferredInvocation.ProtoReflect.Descriptor instead.
func (*DeferredInvocation) Descriptor() ([]byte, []int) {
	return file_std_types_invocation_proto_rawDescGZIP(), []int{1}
}

func (x *DeferredInvocation) GetBinaryPackage() string {
	if x != nil {
		return x.BinaryPackage
	}
	return ""
}

func (x *DeferredInvocation) GetImage() string {
	if x != nil {
		return x.Image
	}
	return ""
}

func (x *DeferredInvocation) GetBinaryConfig() *schema.BinaryConfig {
	if x != nil {
		return x.BinaryConfig
	}
	return nil
}

func (x *DeferredInvocation) GetCacheable() bool {
	if x != nil {
		return x.Cacheable
	}
	return false
}

func (x *DeferredInvocation) GetWithInput() *anypb.Any {
	if x != nil {
		return x.WithInput
	}
	return nil
}

var File_std_types_invocation_proto protoreflect.FileDescriptor

var file_std_types_invocation_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x73, 0x74, 0x64, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f, 0x69, 0x6e, 0x76, 0x6f,
	0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14, 0x66, 0x6f,
	0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x74, 0x64, 0x2e, 0x74, 0x79, 0x70,
	0x65, 0x73, 0x1a, 0x19, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x62, 0x75, 0x66, 0x2f, 0x61, 0x6e, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x17, 0x73,
	0x74, 0x64, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x13, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x62,
	0x69, 0x6e, 0x61, 0x72, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x8b, 0x01, 0x0a, 0x18,
	0x44, 0x65, 0x66, 0x65, 0x72, 0x72, 0x65, 0x64, 0x49, 0x6e, 0x76, 0x6f, 0x63, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x53, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x12, 0x1c, 0x0a, 0x06, 0x62, 0x69, 0x6e, 0x61,
	0x72, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x42, 0x04, 0x80, 0xa6, 0x1d, 0x01, 0x52, 0x06,
	0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x12, 0x1c, 0x0a, 0x09, 0x63, 0x61, 0x63, 0x68, 0x65, 0x61,
	0x62, 0x6c, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x63, 0x61, 0x63, 0x68, 0x65,
	0x61, 0x62, 0x6c, 0x65, 0x12, 0x33, 0x0a, 0x0a, 0x77, 0x69, 0x74, 0x68, 0x5f, 0x69, 0x6e, 0x70,
	0x75, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52, 0x09,
	0x77, 0x69, 0x74, 0x68, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x22, 0xea, 0x01, 0x0a, 0x12, 0x44, 0x65,
	0x66, 0x65, 0x72, 0x72, 0x65, 0x64, 0x49, 0x6e, 0x76, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x25, 0x0a, 0x0e, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x5f, 0x70, 0x61, 0x63, 0x6b, 0x61,
	0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79,
	0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x69, 0x6d, 0x61, 0x67, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x69, 0x6d, 0x61, 0x67, 0x65, 0x12, 0x44, 0x0a,
	0x0d, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x5f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x42, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x43,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x52, 0x0c, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x12, 0x1c, 0x0a, 0x09, 0x63, 0x61, 0x63, 0x68, 0x65, 0x61, 0x62, 0x6c, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x63, 0x61, 0x63, 0x68, 0x65, 0x61, 0x62, 0x6c,
	0x65, 0x12, 0x33, 0x0a, 0x0a, 0x77, 0x69, 0x74, 0x68, 0x5f, 0x69, 0x6e, 0x70, 0x75, 0x74, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52, 0x09, 0x77, 0x69, 0x74,
	0x68, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x42, 0x28, 0x5a, 0x26, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70,
	0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e,
	0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x73, 0x74, 0x64, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_std_types_invocation_proto_rawDescOnce sync.Once
	file_std_types_invocation_proto_rawDescData = file_std_types_invocation_proto_rawDesc
)

func file_std_types_invocation_proto_rawDescGZIP() []byte {
	file_std_types_invocation_proto_rawDescOnce.Do(func() {
		file_std_types_invocation_proto_rawDescData = protoimpl.X.CompressGZIP(file_std_types_invocation_proto_rawDescData)
	})
	return file_std_types_invocation_proto_rawDescData
}

var file_std_types_invocation_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_std_types_invocation_proto_goTypes = []interface{}{
	(*DeferredInvocationSource)(nil), // 0: foundation.std.types.DeferredInvocationSource
	(*DeferredInvocation)(nil),       // 1: foundation.std.types.DeferredInvocation
	(*anypb.Any)(nil),                // 2: google.protobuf.Any
	(*schema.BinaryConfig)(nil),      // 3: foundation.schema.BinaryConfig
}
var file_std_types_invocation_proto_depIdxs = []int32{
	2, // 0: foundation.std.types.DeferredInvocationSource.with_input:type_name -> google.protobuf.Any
	3, // 1: foundation.std.types.DeferredInvocation.binary_config:type_name -> foundation.schema.BinaryConfig
	2, // 2: foundation.std.types.DeferredInvocation.with_input:type_name -> google.protobuf.Any
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_std_types_invocation_proto_init() }
func file_std_types_invocation_proto_init() {
	if File_std_types_invocation_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_std_types_invocation_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeferredInvocationSource); i {
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
		file_std_types_invocation_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeferredInvocation); i {
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
			RawDescriptor: file_std_types_invocation_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_std_types_invocation_proto_goTypes,
		DependencyIndexes: file_std_types_invocation_proto_depIdxs,
		MessageInfos:      file_std_types_invocation_proto_msgTypes,
	}.Build()
	File_std_types_invocation_proto = out.File
	file_std_types_invocation_proto_rawDesc = nil
	file_std_types_invocation_proto_goTypes = nil
	file_std_types_invocation_proto_depIdxs = nil
}
