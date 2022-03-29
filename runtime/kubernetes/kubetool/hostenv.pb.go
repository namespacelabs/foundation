// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: runtime/kubernetes/kubetool/hostenv.proto

package kubetool

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

type KubernetesEnv struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Namespace string `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
}

func (x *KubernetesEnv) Reset() {
	*x = KubernetesEnv{}
	if protoimpl.UnsafeEnabled {
		mi := &file_runtime_kubernetes_kubetool_hostenv_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *KubernetesEnv) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*KubernetesEnv) ProtoMessage() {}

func (x *KubernetesEnv) ProtoReflect() protoreflect.Message {
	mi := &file_runtime_kubernetes_kubetool_hostenv_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use KubernetesEnv.ProtoReflect.Descriptor instead.
func (*KubernetesEnv) Descriptor() ([]byte, []int) {
	return file_runtime_kubernetes_kubetool_hostenv_proto_rawDescGZIP(), []int{0}
}

func (x *KubernetesEnv) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

var File_runtime_kubernetes_kubetool_hostenv_proto protoreflect.FileDescriptor

var file_runtime_kubernetes_kubetool_hostenv_proto_rawDesc = []byte{
	0x0a, 0x29, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2f, 0x6b, 0x75, 0x62, 0x65, 0x72, 0x6e,
	0x65, 0x74, 0x65, 0x73, 0x2f, 0x6b, 0x75, 0x62, 0x65, 0x74, 0x6f, 0x6f, 0x6c, 0x2f, 0x68, 0x6f,
	0x73, 0x74, 0x65, 0x6e, 0x76, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x26, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2e,
	0x6b, 0x75, 0x62, 0x65, 0x72, 0x6e, 0x65, 0x74, 0x65, 0x73, 0x2e, 0x6b, 0x75, 0x62, 0x65, 0x74,
	0x6f, 0x6f, 0x6c, 0x22, 0x2d, 0x0a, 0x0d, 0x4b, 0x75, 0x62, 0x65, 0x72, 0x6e, 0x65, 0x74, 0x65,
	0x73, 0x45, 0x6e, 0x76, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61,
	0x63, 0x65, 0x42, 0x3a, 0x5a, 0x38, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c,
	0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2f, 0x72, 0x75, 0x6e, 0x74, 0x69, 0x6d, 0x65, 0x2f, 0x6b, 0x75, 0x62, 0x65, 0x72,
	0x6e, 0x65, 0x74, 0x65, 0x73, 0x2f, 0x6b, 0x75, 0x62, 0x65, 0x74, 0x6f, 0x6f, 0x6c, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_runtime_kubernetes_kubetool_hostenv_proto_rawDescOnce sync.Once
	file_runtime_kubernetes_kubetool_hostenv_proto_rawDescData = file_runtime_kubernetes_kubetool_hostenv_proto_rawDesc
)

func file_runtime_kubernetes_kubetool_hostenv_proto_rawDescGZIP() []byte {
	file_runtime_kubernetes_kubetool_hostenv_proto_rawDescOnce.Do(func() {
		file_runtime_kubernetes_kubetool_hostenv_proto_rawDescData = protoimpl.X.CompressGZIP(file_runtime_kubernetes_kubetool_hostenv_proto_rawDescData)
	})
	return file_runtime_kubernetes_kubetool_hostenv_proto_rawDescData
}

var file_runtime_kubernetes_kubetool_hostenv_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_runtime_kubernetes_kubetool_hostenv_proto_goTypes = []interface{}{
	(*KubernetesEnv)(nil), // 0: foundation.runtime.kubernetes.kubetool.KubernetesEnv
}
var file_runtime_kubernetes_kubetool_hostenv_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_runtime_kubernetes_kubetool_hostenv_proto_init() }
func file_runtime_kubernetes_kubetool_hostenv_proto_init() {
	if File_runtime_kubernetes_kubetool_hostenv_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_runtime_kubernetes_kubetool_hostenv_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*KubernetesEnv); i {
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
			RawDescriptor: file_runtime_kubernetes_kubetool_hostenv_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_runtime_kubernetes_kubetool_hostenv_proto_goTypes,
		DependencyIndexes: file_runtime_kubernetes_kubetool_hostenv_proto_depIdxs,
		MessageInfos:      file_runtime_kubernetes_kubetool_hostenv_proto_msgTypes,
	}.Build()
	File_runtime_kubernetes_kubetool_hostenv_proto = out.File
	file_runtime_kubernetes_kubetool_hostenv_proto_rawDesc = nil
	file_runtime_kubernetes_kubetool_hostenv_proto_goTypes = nil
	file_runtime_kubernetes_kubetool_hostenv_proto_depIdxs = nil
}