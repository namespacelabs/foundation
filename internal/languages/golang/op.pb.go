// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: internal/languages/golang/op.proto

package golang

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

type OpGenNode struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Node       *schema.Node   `protobuf:"bytes,1,opt,name=node,proto3" json:"node,omitempty"`
	LoadedNode []*schema.Node `protobuf:"bytes,2,rep,name=loaded_node,json=loadedNode,proto3" json:"loaded_node,omitempty"`
}

func (x *OpGenNode) Reset() {
	*x = OpGenNode{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_languages_golang_op_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *OpGenNode) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OpGenNode) ProtoMessage() {}

func (x *OpGenNode) ProtoReflect() protoreflect.Message {
	mi := &file_internal_languages_golang_op_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OpGenNode.ProtoReflect.Descriptor instead.
func (*OpGenNode) Descriptor() ([]byte, []int) {
	return file_internal_languages_golang_op_proto_rawDescGZIP(), []int{0}
}

func (x *OpGenNode) GetNode() *schema.Node {
	if x != nil {
		return x.Node
	}
	return nil
}

func (x *OpGenNode) GetLoadedNode() []*schema.Node {
	if x != nil {
		return x.LoadedNode
	}
	return nil
}

type OpGenServer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Server     *schema.Server `protobuf:"bytes,1,opt,name=server,proto3" json:"server,omitempty"`
	LoadedNode []*schema.Node `protobuf:"bytes,2,rep,name=loaded_node,json=loadedNode,proto3" json:"loaded_node,omitempty"`
}

func (x *OpGenServer) Reset() {
	*x = OpGenServer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_languages_golang_op_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *OpGenServer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OpGenServer) ProtoMessage() {}

func (x *OpGenServer) ProtoReflect() protoreflect.Message {
	mi := &file_internal_languages_golang_op_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OpGenServer.ProtoReflect.Descriptor instead.
func (*OpGenServer) Descriptor() ([]byte, []int) {
	return file_internal_languages_golang_op_proto_rawDescGZIP(), []int{1}
}

func (x *OpGenServer) GetServer() *schema.Server {
	if x != nil {
		return x.Server
	}
	return nil
}

func (x *OpGenServer) GetLoadedNode() []*schema.Node {
	if x != nil {
		return x.LoadedNode
	}
	return nil
}

var File_internal_languages_golang_op_proto protoreflect.FileDescriptor

var file_internal_languages_golang_op_proto_rawDesc = []byte{
	0x0a, 0x22, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x6c, 0x61, 0x6e, 0x67, 0x75,
	0x61, 0x67, 0x65, 0x73, 0x2f, 0x67, 0x6f, 0x6c, 0x61, 0x6e, 0x67, 0x2f, 0x6f, 0x70, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1b, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x2e, 0x6c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x73, 0x2e, 0x67, 0x6f, 0x6c, 0x61, 0x6e,
	0x67, 0x1a, 0x11, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x6e, 0x6f, 0x64, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x13, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x73, 0x65, 0x72,
	0x76, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x72, 0x0a, 0x09, 0x4f, 0x70, 0x47,
	0x65, 0x6e, 0x4e, 0x6f, 0x64, 0x65, 0x12, 0x2b, 0x0a, 0x04, 0x6e, 0x6f, 0x64, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x52, 0x04, 0x6e,
	0x6f, 0x64, 0x65, 0x12, 0x38, 0x0a, 0x0b, 0x6c, 0x6f, 0x61, 0x64, 0x65, 0x64, 0x5f, 0x6e, 0x6f,
	0x64, 0x65, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x4e, 0x6f, 0x64,
	0x65, 0x52, 0x0a, 0x6c, 0x6f, 0x61, 0x64, 0x65, 0x64, 0x4e, 0x6f, 0x64, 0x65, 0x22, 0x7a, 0x0a,
	0x0b, 0x4f, 0x70, 0x47, 0x65, 0x6e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x12, 0x31, 0x0a, 0x06,
	0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x66,
	0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x2e, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x52, 0x06, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x12,
	0x38, 0x0a, 0x0b, 0x6c, 0x6f, 0x61, 0x64, 0x65, 0x64, 0x5f, 0x6e, 0x6f, 0x64, 0x65, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x52, 0x0a, 0x6c,
	0x6f, 0x61, 0x64, 0x65, 0x64, 0x4e, 0x6f, 0x64, 0x65, 0x42, 0x38, 0x5a, 0x36, 0x6e, 0x61, 0x6d,
	0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66,
	0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e,
	0x61, 0x6c, 0x2f, 0x6c, 0x61, 0x6e, 0x67, 0x75, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x67, 0x6f, 0x6c,
	0x61, 0x6e, 0x67, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_internal_languages_golang_op_proto_rawDescOnce sync.Once
	file_internal_languages_golang_op_proto_rawDescData = file_internal_languages_golang_op_proto_rawDesc
)

func file_internal_languages_golang_op_proto_rawDescGZIP() []byte {
	file_internal_languages_golang_op_proto_rawDescOnce.Do(func() {
		file_internal_languages_golang_op_proto_rawDescData = protoimpl.X.CompressGZIP(file_internal_languages_golang_op_proto_rawDescData)
	})
	return file_internal_languages_golang_op_proto_rawDescData
}

var file_internal_languages_golang_op_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_internal_languages_golang_op_proto_goTypes = []interface{}{
	(*OpGenNode)(nil),     // 0: foundation.languages.golang.OpGenNode
	(*OpGenServer)(nil),   // 1: foundation.languages.golang.OpGenServer
	(*schema.Node)(nil),   // 2: foundation.schema.Node
	(*schema.Server)(nil), // 3: foundation.schema.Server
}
var file_internal_languages_golang_op_proto_depIdxs = []int32{
	2, // 0: foundation.languages.golang.OpGenNode.node:type_name -> foundation.schema.Node
	2, // 1: foundation.languages.golang.OpGenNode.loaded_node:type_name -> foundation.schema.Node
	3, // 2: foundation.languages.golang.OpGenServer.server:type_name -> foundation.schema.Server
	2, // 3: foundation.languages.golang.OpGenServer.loaded_node:type_name -> foundation.schema.Node
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	4, // [4:4] is the sub-list for extension extendee
	0, // [0:4] is the sub-list for field type_name
}

func init() { file_internal_languages_golang_op_proto_init() }
func file_internal_languages_golang_op_proto_init() {
	if File_internal_languages_golang_op_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_internal_languages_golang_op_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*OpGenNode); i {
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
		file_internal_languages_golang_op_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*OpGenServer); i {
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
			RawDescriptor: file_internal_languages_golang_op_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_internal_languages_golang_op_proto_goTypes,
		DependencyIndexes: file_internal_languages_golang_op_proto_depIdxs,
		MessageInfos:      file_internal_languages_golang_op_proto_msgTypes,
	}.Build()
	File_internal_languages_golang_op_proto = out.File
	file_internal_languages_golang_op_proto_rawDesc = nil
	file_internal_languages_golang_op_proto_goTypes = nil
	file_internal_languages_golang_op_proto_depIdxs = nil
}