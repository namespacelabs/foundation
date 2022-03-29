// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: std/dev/controller/admin/config.proto

package admin

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

	Backend      []*Backend `protobuf:"bytes,1,rep,name=backend,proto3" json:"backend,omitempty"`
	PackageBase  string     `protobuf:"bytes,2,opt,name=package_base,json=packageBase,proto3" json:"package_base,omitempty"`
	FilesyncPort int32      `protobuf:"varint,3,opt,name=filesync_port,json=filesyncPort,proto3" json:"filesync_port,omitempty"`
	RevproxyPort int32      `protobuf:"varint,4,opt,name=revproxy_port,json=revproxyPort,proto3" json:"revproxy_port,omitempty"`
}

func (x *Configuration) Reset() {
	*x = Configuration{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_dev_controller_admin_config_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Configuration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Configuration) ProtoMessage() {}

func (x *Configuration) ProtoReflect() protoreflect.Message {
	mi := &file_std_dev_controller_admin_config_proto_msgTypes[0]
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
	return file_std_dev_controller_admin_config_proto_rawDescGZIP(), []int{0}
}

func (x *Configuration) GetBackend() []*Backend {
	if x != nil {
		return x.Backend
	}
	return nil
}

func (x *Configuration) GetPackageBase() string {
	if x != nil {
		return x.PackageBase
	}
	return ""
}

func (x *Configuration) GetFilesyncPort() int32 {
	if x != nil {
		return x.FilesyncPort
	}
	return 0
}

func (x *Configuration) GetRevproxyPort() int32 {
	if x != nil {
		return x.RevproxyPort
	}
	return 0
}

type Backend struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PackageName string              `protobuf:"bytes,1,opt,name=package_name,json=packageName,proto3" json:"package_name,omitempty"`
	Execution   *Execution          `protobuf:"bytes,2,opt,name=execution,proto3" json:"execution,omitempty"`
	HttpPass    *HttpPass           `protobuf:"bytes,3,opt,name=http_pass,json=httpPass,proto3" json:"http_pass,omitempty"`
	OnChange    []*Backend_OnChange `protobuf:"bytes,4,rep,name=on_change,json=onChange,proto3" json:"on_change,omitempty"`
}

func (x *Backend) Reset() {
	*x = Backend{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_dev_controller_admin_config_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Backend) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Backend) ProtoMessage() {}

func (x *Backend) ProtoReflect() protoreflect.Message {
	mi := &file_std_dev_controller_admin_config_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Backend.ProtoReflect.Descriptor instead.
func (*Backend) Descriptor() ([]byte, []int) {
	return file_std_dev_controller_admin_config_proto_rawDescGZIP(), []int{1}
}

func (x *Backend) GetPackageName() string {
	if x != nil {
		return x.PackageName
	}
	return ""
}

func (x *Backend) GetExecution() *Execution {
	if x != nil {
		return x.Execution
	}
	return nil
}

func (x *Backend) GetHttpPass() *HttpPass {
	if x != nil {
		return x.HttpPass
	}
	return nil
}

func (x *Backend) GetOnChange() []*Backend_OnChange {
	if x != nil {
		return x.OnChange
	}
	return nil
}

type Execution struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Args          []string `protobuf:"bytes,1,rep,name=args,proto3" json:"args,omitempty"`
	AdditionalEnv []string `protobuf:"bytes,2,rep,name=additional_env,json=additionalEnv,proto3" json:"additional_env,omitempty"`
}

func (x *Execution) Reset() {
	*x = Execution{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_dev_controller_admin_config_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Execution) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Execution) ProtoMessage() {}

func (x *Execution) ProtoReflect() protoreflect.Message {
	mi := &file_std_dev_controller_admin_config_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Execution.ProtoReflect.Descriptor instead.
func (*Execution) Descriptor() ([]byte, []int) {
	return file_std_dev_controller_admin_config_proto_rawDescGZIP(), []int{2}
}

func (x *Execution) GetArgs() []string {
	if x != nil {
		return x.Args
	}
	return nil
}

func (x *Execution) GetAdditionalEnv() []string {
	if x != nil {
		return x.AdditionalEnv
	}
	return nil
}

type HttpPass struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	UrlPrefix string `protobuf:"bytes,1,opt,name=url_prefix,json=urlPrefix,proto3" json:"url_prefix,omitempty"`
	Backend   string `protobuf:"bytes,2,opt,name=backend,proto3" json:"backend,omitempty"`
}

func (x *HttpPass) Reset() {
	*x = HttpPass{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_dev_controller_admin_config_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *HttpPass) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*HttpPass) ProtoMessage() {}

func (x *HttpPass) ProtoReflect() protoreflect.Message {
	mi := &file_std_dev_controller_admin_config_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use HttpPass.ProtoReflect.Descriptor instead.
func (*HttpPass) Descriptor() ([]byte, []int) {
	return file_std_dev_controller_admin_config_proto_rawDescGZIP(), []int{3}
}

func (x *HttpPass) GetUrlPrefix() string {
	if x != nil {
		return x.UrlPrefix
	}
	return ""
}

func (x *HttpPass) GetBackend() string {
	if x != nil {
		return x.Backend
	}
	return ""
}

type Backend_OnChange struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Path                  []string   `protobuf:"bytes,1,rep,name=path,proto3" json:"path,omitempty"`
	Execution             *Execution `protobuf:"bytes,2,opt,name=execution,proto3" json:"execution,omitempty"`                                                         // If one of the files above changes, execute this command.
	RestartAfterExecution bool       `protobuf:"varint,3,opt,name=restart_after_execution,json=restartAfterExecution,proto3" json:"restart_after_execution,omitempty"` // If true, restarts the backend after this hook executes.
}

func (x *Backend_OnChange) Reset() {
	*x = Backend_OnChange{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_dev_controller_admin_config_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Backend_OnChange) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Backend_OnChange) ProtoMessage() {}

func (x *Backend_OnChange) ProtoReflect() protoreflect.Message {
	mi := &file_std_dev_controller_admin_config_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Backend_OnChange.ProtoReflect.Descriptor instead.
func (*Backend_OnChange) Descriptor() ([]byte, []int) {
	return file_std_dev_controller_admin_config_proto_rawDescGZIP(), []int{1, 0}
}

func (x *Backend_OnChange) GetPath() []string {
	if x != nil {
		return x.Path
	}
	return nil
}

func (x *Backend_OnChange) GetExecution() *Execution {
	if x != nil {
		return x.Execution
	}
	return nil
}

func (x *Backend_OnChange) GetRestartAfterExecution() bool {
	if x != nil {
		return x.RestartAfterExecution
	}
	return false
}

var File_std_dev_controller_admin_config_proto protoreflect.FileDescriptor

var file_std_dev_controller_admin_config_proto_rawDesc = []byte{
	0x0a, 0x25, 0x73, 0x74, 0x64, 0x2f, 0x64, 0x65, 0x76, 0x2f, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f,
	0x6c, 0x6c, 0x65, 0x72, 0x2f, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x23, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x74, 0x64, 0x2e, 0x64, 0x65, 0x76, 0x2e, 0x63, 0x6f, 0x6e, 0x74,
	0x72, 0x6f, 0x6c, 0x6c, 0x65, 0x72, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x22, 0xc4, 0x01, 0x0a,
	0x0d, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x46,
	0x0a, 0x07, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x2c, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x74, 0x64,
	0x2e, 0x64, 0x65, 0x76, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x6c, 0x65, 0x72, 0x2e,
	0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x52, 0x07, 0x62,
	0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x12, 0x21, 0x0a, 0x0c, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67,
	0x65, 0x5f, 0x62, 0x61, 0x73, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x70, 0x61,
	0x63, 0x6b, 0x61, 0x67, 0x65, 0x42, 0x61, 0x73, 0x65, 0x12, 0x23, 0x0a, 0x0d, 0x66, 0x69, 0x6c,
	0x65, 0x73, 0x79, 0x6e, 0x63, 0x5f, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05,
	0x52, 0x0c, 0x66, 0x69, 0x6c, 0x65, 0x73, 0x79, 0x6e, 0x63, 0x50, 0x6f, 0x72, 0x74, 0x12, 0x23,
	0x0a, 0x0d, 0x72, 0x65, 0x76, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x5f, 0x70, 0x6f, 0x72, 0x74, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0c, 0x72, 0x65, 0x76, 0x70, 0x72, 0x6f, 0x78, 0x79, 0x50,
	0x6f, 0x72, 0x74, 0x22, 0xc1, 0x03, 0x0a, 0x07, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x12,
	0x21, 0x0a, 0x0c, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x4e, 0x61,
	0x6d, 0x65, 0x12, 0x4c, 0x0a, 0x09, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2e, 0x73, 0x74, 0x64, 0x2e, 0x64, 0x65, 0x76, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72,
	0x6f, 0x6c, 0x6c, 0x65, 0x72, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x45, 0x78, 0x65, 0x63,
	0x75, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x09, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x4a, 0x0a, 0x09, 0x68, 0x74, 0x74, 0x70, 0x5f, 0x70, 0x61, 0x73, 0x73, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x2d, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x2e, 0x73, 0x74, 0x64, 0x2e, 0x64, 0x65, 0x76, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c,
	0x6c, 0x65, 0x72, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x48, 0x74, 0x74, 0x70, 0x50, 0x61,
	0x73, 0x73, 0x52, 0x08, 0x68, 0x74, 0x74, 0x70, 0x50, 0x61, 0x73, 0x73, 0x12, 0x52, 0x0a, 0x09,
	0x6f, 0x6e, 0x5f, 0x63, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x35, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x74, 0x64,
	0x2e, 0x64, 0x65, 0x76, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x6c, 0x65, 0x72, 0x2e,
	0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x4f, 0x6e,
	0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x52, 0x08, 0x6f, 0x6e, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65,
	0x1a, 0xa4, 0x01, 0x0a, 0x08, 0x4f, 0x6e, 0x43, 0x68, 0x61, 0x6e, 0x67, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x70, 0x61, 0x74, 0x68, 0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x70, 0x61, 0x74,
	0x68, 0x12, 0x4c, 0x0a, 0x09, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x2e, 0x73, 0x74, 0x64, 0x2e, 0x64, 0x65, 0x76, 0x2e, 0x63, 0x6f, 0x6e, 0x74, 0x72, 0x6f,
	0x6c, 0x6c, 0x65, 0x72, 0x2e, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x2e, 0x45, 0x78, 0x65, 0x63, 0x75,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x09, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x12,
	0x36, 0x0a, 0x17, 0x72, 0x65, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x61, 0x66, 0x74, 0x65, 0x72,
	0x5f, 0x65, 0x78, 0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x15, 0x72, 0x65, 0x73, 0x74, 0x61, 0x72, 0x74, 0x41, 0x66, 0x74, 0x65, 0x72, 0x45, 0x78,
	0x65, 0x63, 0x75, 0x74, 0x69, 0x6f, 0x6e, 0x22, 0x46, 0x0a, 0x09, 0x45, 0x78, 0x65, 0x63, 0x75,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x61, 0x72, 0x67, 0x73, 0x18, 0x01, 0x20, 0x03,
	0x28, 0x09, 0x52, 0x04, 0x61, 0x72, 0x67, 0x73, 0x12, 0x25, 0x0a, 0x0e, 0x61, 0x64, 0x64, 0x69,
	0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x5f, 0x65, 0x6e, 0x76, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09,
	0x52, 0x0d, 0x61, 0x64, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x45, 0x6e, 0x76, 0x22,
	0x43, 0x0a, 0x08, 0x48, 0x74, 0x74, 0x70, 0x50, 0x61, 0x73, 0x73, 0x12, 0x1d, 0x0a, 0x0a, 0x75,
	0x72, 0x6c, 0x5f, 0x70, 0x72, 0x65, 0x66, 0x69, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x09, 0x75, 0x72, 0x6c, 0x50, 0x72, 0x65, 0x66, 0x69, 0x78, 0x12, 0x18, 0x0a, 0x07, 0x62, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x62, 0x61, 0x63,
	0x6b, 0x65, 0x6e, 0x64, 0x42, 0x37, 0x5a, 0x35, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x73, 0x74, 0x64, 0x2f, 0x64, 0x65, 0x76, 0x2f, 0x63, 0x6f, 0x6e,
	0x74, 0x72, 0x6f, 0x6c, 0x6c, 0x65, 0x72, 0x2f, 0x61, 0x64, 0x6d, 0x69, 0x6e, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_std_dev_controller_admin_config_proto_rawDescOnce sync.Once
	file_std_dev_controller_admin_config_proto_rawDescData = file_std_dev_controller_admin_config_proto_rawDesc
)

func file_std_dev_controller_admin_config_proto_rawDescGZIP() []byte {
	file_std_dev_controller_admin_config_proto_rawDescOnce.Do(func() {
		file_std_dev_controller_admin_config_proto_rawDescData = protoimpl.X.CompressGZIP(file_std_dev_controller_admin_config_proto_rawDescData)
	})
	return file_std_dev_controller_admin_config_proto_rawDescData
}

var file_std_dev_controller_admin_config_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_std_dev_controller_admin_config_proto_goTypes = []interface{}{
	(*Configuration)(nil),    // 0: foundation.std.dev.controller.admin.Configuration
	(*Backend)(nil),          // 1: foundation.std.dev.controller.admin.Backend
	(*Execution)(nil),        // 2: foundation.std.dev.controller.admin.Execution
	(*HttpPass)(nil),         // 3: foundation.std.dev.controller.admin.HttpPass
	(*Backend_OnChange)(nil), // 4: foundation.std.dev.controller.admin.Backend.OnChange
}
var file_std_dev_controller_admin_config_proto_depIdxs = []int32{
	1, // 0: foundation.std.dev.controller.admin.Configuration.backend:type_name -> foundation.std.dev.controller.admin.Backend
	2, // 1: foundation.std.dev.controller.admin.Backend.execution:type_name -> foundation.std.dev.controller.admin.Execution
	3, // 2: foundation.std.dev.controller.admin.Backend.http_pass:type_name -> foundation.std.dev.controller.admin.HttpPass
	4, // 3: foundation.std.dev.controller.admin.Backend.on_change:type_name -> foundation.std.dev.controller.admin.Backend.OnChange
	2, // 4: foundation.std.dev.controller.admin.Backend.OnChange.execution:type_name -> foundation.std.dev.controller.admin.Execution
	5, // [5:5] is the sub-list for method output_type
	5, // [5:5] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_std_dev_controller_admin_config_proto_init() }
func file_std_dev_controller_admin_config_proto_init() {
	if File_std_dev_controller_admin_config_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_std_dev_controller_admin_config_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
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
		file_std_dev_controller_admin_config_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Backend); i {
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
		file_std_dev_controller_admin_config_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Execution); i {
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
		file_std_dev_controller_admin_config_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*HttpPass); i {
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
		file_std_dev_controller_admin_config_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Backend_OnChange); i {
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
			RawDescriptor: file_std_dev_controller_admin_config_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_std_dev_controller_admin_config_proto_goTypes,
		DependencyIndexes: file_std_dev_controller_admin_config_proto_depIdxs,
		MessageInfos:      file_std_dev_controller_admin_config_proto_msgTypes,
	}.Build()
	File_std_dev_controller_admin_config_proto = out.File
	file_std_dev_controller_admin_config_proto_rawDesc = nil
	file_std_dev_controller_admin_config_proto_goTypes = nil
	file_std_dev_controller_admin_config_proto_depIdxs = nil
}