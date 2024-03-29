// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: internal/wscontents/types.proto

package wscontents

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

type FileEvent_EventType int32

const (
	FileEvent_UNKNOWN FileEvent_EventType = 0
	FileEvent_WRITE   FileEvent_EventType = 1
	FileEvent_REMOVE  FileEvent_EventType = 2
	FileEvent_MKDIR   FileEvent_EventType = 3
)

// Enum value maps for FileEvent_EventType.
var (
	FileEvent_EventType_name = map[int32]string{
		0: "UNKNOWN",
		1: "WRITE",
		2: "REMOVE",
		3: "MKDIR",
	}
	FileEvent_EventType_value = map[string]int32{
		"UNKNOWN": 0,
		"WRITE":   1,
		"REMOVE":  2,
		"MKDIR":   3,
	}
)

func (x FileEvent_EventType) Enum() *FileEvent_EventType {
	p := new(FileEvent_EventType)
	*p = x
	return p
}

func (x FileEvent_EventType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (FileEvent_EventType) Descriptor() protoreflect.EnumDescriptor {
	return file_internal_wscontents_types_proto_enumTypes[0].Descriptor()
}

func (FileEvent_EventType) Type() protoreflect.EnumType {
	return &file_internal_wscontents_types_proto_enumTypes[0]
}

func (x FileEvent_EventType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use FileEvent_EventType.Descriptor instead.
func (FileEvent_EventType) EnumDescriptor() ([]byte, []int) {
	return file_internal_wscontents_types_proto_rawDescGZIP(), []int{0, 0}
}

type FileEvent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Event       FileEvent_EventType `protobuf:"varint,1,opt,name=event,proto3,enum=foundation.internal.wscontents.FileEvent_EventType" json:"event,omitempty"`
	Path        string              `protobuf:"bytes,2,opt,name=path,proto3" json:"path,omitempty"`
	NewContents []byte              `protobuf:"bytes,3,opt,name=new_contents,json=newContents,proto3" json:"new_contents,omitempty"`
	Mode        uint32              `protobuf:"varint,4,opt,name=mode,proto3" json:"mode,omitempty"`
}

func (x *FileEvent) Reset() {
	*x = FileEvent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_internal_wscontents_types_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FileEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FileEvent) ProtoMessage() {}

func (x *FileEvent) ProtoReflect() protoreflect.Message {
	mi := &file_internal_wscontents_types_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FileEvent.ProtoReflect.Descriptor instead.
func (*FileEvent) Descriptor() ([]byte, []int) {
	return file_internal_wscontents_types_proto_rawDescGZIP(), []int{0}
}

func (x *FileEvent) GetEvent() FileEvent_EventType {
	if x != nil {
		return x.Event
	}
	return FileEvent_UNKNOWN
}

func (x *FileEvent) GetPath() string {
	if x != nil {
		return x.Path
	}
	return ""
}

func (x *FileEvent) GetNewContents() []byte {
	if x != nil {
		return x.NewContents
	}
	return nil
}

func (x *FileEvent) GetMode() uint32 {
	if x != nil {
		return x.Mode
	}
	return 0
}

var File_internal_wscontents_types_proto protoreflect.FileDescriptor

var file_internal_wscontents_types_proto_rawDesc = []byte{
	0x0a, 0x1f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x77, 0x73, 0x63, 0x6f, 0x6e,
	0x74, 0x65, 0x6e, 0x74, 0x73, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x1e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x69, 0x6e,
	0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x77, 0x73, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74,
	0x73, 0x22, 0xdd, 0x01, 0x0a, 0x09, 0x46, 0x69, 0x6c, 0x65, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x12,
	0x49, 0x0a, 0x05, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x33,
	0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x69, 0x6e, 0x74, 0x65,
	0x72, 0x6e, 0x61, 0x6c, 0x2e, 0x77, 0x73, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x2e,
	0x46, 0x69, 0x6c, 0x65, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x2e, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x54,
	0x79, 0x70, 0x65, 0x52, 0x05, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61,
	0x74, 0x68, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74, 0x68, 0x12, 0x21,
	0x0a, 0x0c, 0x6e, 0x65, 0x77, 0x5f, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x6e, 0x65, 0x77, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74,
	0x73, 0x12, 0x12, 0x0a, 0x04, 0x6d, 0x6f, 0x64, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52,
	0x04, 0x6d, 0x6f, 0x64, 0x65, 0x22, 0x3a, 0x0a, 0x09, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x54, 0x79,
	0x70, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00, 0x12,
	0x09, 0x0a, 0x05, 0x57, 0x52, 0x49, 0x54, 0x45, 0x10, 0x01, 0x12, 0x0a, 0x0a, 0x06, 0x52, 0x45,
	0x4d, 0x4f, 0x56, 0x45, 0x10, 0x02, 0x12, 0x09, 0x0a, 0x05, 0x4d, 0x4b, 0x44, 0x49, 0x52, 0x10,
	0x03, 0x42, 0x32, 0x5a, 0x30, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61,
	0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x77, 0x73, 0x63, 0x6f, 0x6e,
	0x74, 0x65, 0x6e, 0x74, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_internal_wscontents_types_proto_rawDescOnce sync.Once
	file_internal_wscontents_types_proto_rawDescData = file_internal_wscontents_types_proto_rawDesc
)

func file_internal_wscontents_types_proto_rawDescGZIP() []byte {
	file_internal_wscontents_types_proto_rawDescOnce.Do(func() {
		file_internal_wscontents_types_proto_rawDescData = protoimpl.X.CompressGZIP(file_internal_wscontents_types_proto_rawDescData)
	})
	return file_internal_wscontents_types_proto_rawDescData
}

var file_internal_wscontents_types_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_internal_wscontents_types_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_internal_wscontents_types_proto_goTypes = []interface{}{
	(FileEvent_EventType)(0), // 0: foundation.internal.wscontents.FileEvent.EventType
	(*FileEvent)(nil),        // 1: foundation.internal.wscontents.FileEvent
}
var file_internal_wscontents_types_proto_depIdxs = []int32{
	0, // 0: foundation.internal.wscontents.FileEvent.event:type_name -> foundation.internal.wscontents.FileEvent.EventType
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_internal_wscontents_types_proto_init() }
func file_internal_wscontents_types_proto_init() {
	if File_internal_wscontents_types_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_internal_wscontents_types_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FileEvent); i {
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
			RawDescriptor: file_internal_wscontents_types_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_internal_wscontents_types_proto_goTypes,
		DependencyIndexes: file_internal_wscontents_types_proto_depIdxs,
		EnumInfos:         file_internal_wscontents_types_proto_enumTypes,
		MessageInfos:      file_internal_wscontents_types_proto_msgTypes,
	}.Build()
	File_internal_wscontents_types_proto = out.File
	file_internal_wscontents_types_proto_rawDesc = nil
	file_internal_wscontents_types_proto_goTypes = nil
	file_internal_wscontents_types_proto_depIdxs = nil
}
