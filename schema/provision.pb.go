// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: schema/provision.proto

package schema

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

type Invocation struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Deprecated, use binary_ref
	Binary       string                         `protobuf:"bytes,1,opt,name=binary,proto3" json:"binary,omitempty"`
	BinaryRef    *PackageRef                    `protobuf:"bytes,9,opt,name=binary_ref,json=binaryRef,proto3" json:"binary_ref,omitempty"`
	Args         []string                       `protobuf:"bytes,2,rep,name=args,proto3" json:"args,omitempty"`
	WorkingDir   string                         `protobuf:"bytes,4,opt,name=working_dir,json=workingDir,proto3" json:"working_dir,omitempty"`
	Snapshots    map[string]*InvocationSnapshot `protobuf:"bytes,5,rep,name=snapshots,proto3" json:"snapshots,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	NoCache      bool                           `protobuf:"varint,6,opt,name=no_cache,json=noCache,proto3" json:"no_cache,omitempty"`
	RequiresKeys bool                           `protobuf:"varint,7,opt,name=requires_keys,json=requiresKeys,proto3" json:"requires_keys,omitempty"`
	Inject       []*Invocation_ValueInjection   `protobuf:"bytes,8,rep,name=inject,proto3" json:"inject,omitempty"`
}

func (x *Invocation) Reset() {
	*x = Invocation{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_provision_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Invocation) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Invocation) ProtoMessage() {}

func (x *Invocation) ProtoReflect() protoreflect.Message {
	mi := &file_schema_provision_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Invocation.ProtoReflect.Descriptor instead.
func (*Invocation) Descriptor() ([]byte, []int) {
	return file_schema_provision_proto_rawDescGZIP(), []int{0}
}

func (x *Invocation) GetBinary() string {
	if x != nil {
		return x.Binary
	}
	return ""
}

func (x *Invocation) GetBinaryRef() *PackageRef {
	if x != nil {
		return x.BinaryRef
	}
	return nil
}

func (x *Invocation) GetArgs() []string {
	if x != nil {
		return x.Args
	}
	return nil
}

func (x *Invocation) GetWorkingDir() string {
	if x != nil {
		return x.WorkingDir
	}
	return ""
}

func (x *Invocation) GetSnapshots() map[string]*InvocationSnapshot {
	if x != nil {
		return x.Snapshots
	}
	return nil
}

func (x *Invocation) GetNoCache() bool {
	if x != nil {
		return x.NoCache
	}
	return false
}

func (x *Invocation) GetRequiresKeys() bool {
	if x != nil {
		return x.RequiresKeys
	}
	return false
}

func (x *Invocation) GetInject() []*Invocation_ValueInjection {
	if x != nil {
		return x.Inject
	}
	return nil
}

type InvocationSnapshot struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	FromWorkspace string `protobuf:"bytes,1,opt,name=from_workspace,json=fromWorkspace,proto3" json:"from_workspace,omitempty"`
	Optional      bool   `protobuf:"varint,2,opt,name=optional,proto3" json:"optional,omitempty"`
	RequireFile   bool   `protobuf:"varint,3,opt,name=require_file,json=requireFile,proto3" json:"require_file,omitempty"`
}

func (x *InvocationSnapshot) Reset() {
	*x = InvocationSnapshot{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_provision_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InvocationSnapshot) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InvocationSnapshot) ProtoMessage() {}

func (x *InvocationSnapshot) ProtoReflect() protoreflect.Message {
	mi := &file_schema_provision_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InvocationSnapshot.ProtoReflect.Descriptor instead.
func (*InvocationSnapshot) Descriptor() ([]byte, []int) {
	return file_schema_provision_proto_rawDescGZIP(), []int{1}
}

func (x *InvocationSnapshot) GetFromWorkspace() string {
	if x != nil {
		return x.FromWorkspace
	}
	return ""
}

func (x *InvocationSnapshot) GetOptional() bool {
	if x != nil {
		return x.Optional
	}
	return false
}

func (x *InvocationSnapshot) GetRequireFile() bool {
	if x != nil {
		return x.RequireFile
	}
	return false
}

type StartupPlan struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Args []string          `protobuf:"bytes,1,rep,name=args,proto3" json:"args,omitempty"`
	Env  map[string]string `protobuf:"bytes,2,rep,name=env,proto3" json:"env,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *StartupPlan) Reset() {
	*x = StartupPlan{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_provision_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StartupPlan) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StartupPlan) ProtoMessage() {}

func (x *StartupPlan) ProtoReflect() protoreflect.Message {
	mi := &file_schema_provision_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StartupPlan.ProtoReflect.Descriptor instead.
func (*StartupPlan) Descriptor() ([]byte, []int) {
	return file_schema_provision_proto_rawDescGZIP(), []int{2}
}

func (x *StartupPlan) GetArgs() []string {
	if x != nil {
		return x.Args
	}
	return nil
}

func (x *StartupPlan) GetEnv() map[string]string {
	if x != nil {
		return x.Env
	}
	return nil
}

type Invocation_ValueInjection struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Type string `protobuf:"bytes,1,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *Invocation_ValueInjection) Reset() {
	*x = Invocation_ValueInjection{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_provision_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Invocation_ValueInjection) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Invocation_ValueInjection) ProtoMessage() {}

func (x *Invocation_ValueInjection) ProtoReflect() protoreflect.Message {
	mi := &file_schema_provision_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Invocation_ValueInjection.ProtoReflect.Descriptor instead.
func (*Invocation_ValueInjection) Descriptor() ([]byte, []int) {
	return file_schema_provision_proto_rawDescGZIP(), []int{0, 1}
}

func (x *Invocation_ValueInjection) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

var File_schema_provision_proto protoreflect.FileDescriptor

var file_schema_provision_proto_rawDesc = []byte{
	0x0a, 0x16, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x73, 0x69,
	0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x11, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x1a, 0x14, 0x73, 0x63, 0x68,
	0x65, 0x6d, 0x61, 0x2f, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0xfa, 0x03, 0x0a, 0x0a, 0x49, 0x6e, 0x76, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x16, 0x0a, 0x06, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x62, 0x69, 0x6e, 0x61, 0x72, 0x79, 0x12, 0x3c, 0x0a, 0x0a, 0x62, 0x69, 0x6e, 0x61,
	0x72, 0x79, 0x5f, 0x72, 0x65, 0x66, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x66,
	0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x2e, 0x50, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x52, 0x65, 0x66, 0x52, 0x09, 0x62, 0x69, 0x6e,
	0x61, 0x72, 0x79, 0x52, 0x65, 0x66, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x72, 0x67, 0x73, 0x18, 0x02,
	0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x61, 0x72, 0x67, 0x73, 0x12, 0x1f, 0x0a, 0x0b, 0x77, 0x6f,
	0x72, 0x6b, 0x69, 0x6e, 0x67, 0x5f, 0x64, 0x69, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0a, 0x77, 0x6f, 0x72, 0x6b, 0x69, 0x6e, 0x67, 0x44, 0x69, 0x72, 0x12, 0x4a, 0x0a, 0x09, 0x73,
	0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2c,
	0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65,
	0x6d, 0x61, 0x2e, 0x49, 0x6e, 0x76, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x53, 0x6e,
	0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x09, 0x73, 0x6e,
	0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x73, 0x12, 0x19, 0x0a, 0x08, 0x6e, 0x6f, 0x5f, 0x63, 0x61,
	0x63, 0x68, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x6e, 0x6f, 0x43, 0x61, 0x63,
	0x68, 0x65, 0x12, 0x23, 0x0a, 0x0d, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x73, 0x5f, 0x6b,
	0x65, 0x79, 0x73, 0x18, 0x07, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0c, 0x72, 0x65, 0x71, 0x75, 0x69,
	0x72, 0x65, 0x73, 0x4b, 0x65, 0x79, 0x73, 0x12, 0x44, 0x0a, 0x06, 0x69, 0x6e, 0x6a, 0x65, 0x63,
	0x74, 0x18, 0x08, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x49, 0x6e, 0x76, 0x6f,
	0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x49, 0x6e, 0x6a, 0x65,
	0x63, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x06, 0x69, 0x6e, 0x6a, 0x65, 0x63, 0x74, 0x1a, 0x63, 0x0a,
	0x0e, 0x53, 0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x3b, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x25, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63,
	0x68, 0x65, 0x6d, 0x61, 0x2e, 0x49, 0x6e, 0x76, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x53,
	0x6e, 0x61, 0x70, 0x73, 0x68, 0x6f, 0x74, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02,
	0x38, 0x01, 0x1a, 0x24, 0x0a, 0x0e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x49, 0x6e, 0x6a, 0x65, 0x63,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x4a, 0x04, 0x08, 0x03, 0x10, 0x04, 0x22, 0x7a,
	0x0a, 0x12, 0x49, 0x6e, 0x76, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x6e, 0x61, 0x70,
	0x73, 0x68, 0x6f, 0x74, 0x12, 0x25, 0x0a, 0x0e, 0x66, 0x72, 0x6f, 0x6d, 0x5f, 0x77, 0x6f, 0x72,
	0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x66, 0x72,
	0x6f, 0x6d, 0x57, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x6f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x08, 0x6f,
	0x70, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x12, 0x21, 0x0a, 0x0c, 0x72, 0x65, 0x71, 0x75, 0x69,
	0x72, 0x65, 0x5f, 0x66, 0x69, 0x6c, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0b, 0x72,
	0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x46, 0x69, 0x6c, 0x65, 0x22, 0x94, 0x01, 0x0a, 0x0b, 0x53,
	0x74, 0x61, 0x72, 0x74, 0x75, 0x70, 0x50, 0x6c, 0x61, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x72,
	0x67, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x61, 0x72, 0x67, 0x73, 0x12, 0x39,
	0x0a, 0x03, 0x65, 0x6e, 0x76, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x27, 0x2e, 0x66, 0x6f,
	0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e,
	0x53, 0x74, 0x61, 0x72, 0x74, 0x75, 0x70, 0x50, 0x6c, 0x61, 0x6e, 0x2e, 0x45, 0x6e, 0x76, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x03, 0x65, 0x6e, 0x76, 0x1a, 0x36, 0x0a, 0x08, 0x45, 0x6e, 0x76,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38,
	0x01, 0x42, 0x25, 0x5a, 0x23, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61,
	0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_schema_provision_proto_rawDescOnce sync.Once
	file_schema_provision_proto_rawDescData = file_schema_provision_proto_rawDesc
)

func file_schema_provision_proto_rawDescGZIP() []byte {
	file_schema_provision_proto_rawDescOnce.Do(func() {
		file_schema_provision_proto_rawDescData = protoimpl.X.CompressGZIP(file_schema_provision_proto_rawDescData)
	})
	return file_schema_provision_proto_rawDescData
}

var file_schema_provision_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_schema_provision_proto_goTypes = []interface{}{
	(*Invocation)(nil),                // 0: foundation.schema.Invocation
	(*InvocationSnapshot)(nil),        // 1: foundation.schema.InvocationSnapshot
	(*StartupPlan)(nil),               // 2: foundation.schema.StartupPlan
	nil,                               // 3: foundation.schema.Invocation.SnapshotsEntry
	(*Invocation_ValueInjection)(nil), // 4: foundation.schema.Invocation.ValueInjection
	nil,                               // 5: foundation.schema.StartupPlan.EnvEntry
	(*PackageRef)(nil),                // 6: foundation.schema.PackageRef
}
var file_schema_provision_proto_depIdxs = []int32{
	6, // 0: foundation.schema.Invocation.binary_ref:type_name -> foundation.schema.PackageRef
	3, // 1: foundation.schema.Invocation.snapshots:type_name -> foundation.schema.Invocation.SnapshotsEntry
	4, // 2: foundation.schema.Invocation.inject:type_name -> foundation.schema.Invocation.ValueInjection
	5, // 3: foundation.schema.StartupPlan.env:type_name -> foundation.schema.StartupPlan.EnvEntry
	1, // 4: foundation.schema.Invocation.SnapshotsEntry.value:type_name -> foundation.schema.InvocationSnapshot
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_schema_provision_proto_init() }
func file_schema_provision_proto_init() {
	if File_schema_provision_proto != nil {
		return
	}
	file_schema_package_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_schema_provision_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Invocation); i {
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
		file_schema_provision_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InvocationSnapshot); i {
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
		file_schema_provision_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StartupPlan); i {
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
		file_schema_provision_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Invocation_ValueInjection); i {
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
			RawDescriptor: file_schema_provision_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_schema_provision_proto_goTypes,
		DependencyIndexes: file_schema_provision_proto_depIdxs,
		MessageInfos:      file_schema_provision_proto_msgTypes,
	}.Build()
	File_schema_provision_proto = out.File
	file_schema_provision_proto_rawDesc = nil
	file_schema_provision_proto_goTypes = nil
	file_schema_provision_proto_depIdxs = nil
}
