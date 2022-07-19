// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: schema/storage/deploy.proto

package storage

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

type NetworkPlan struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Endpoint           []*NetworkPlan_Endpoint `protobuf:"bytes,1,rep,name=endpoint,proto3" json:"endpoint,omitempty"`
	Ingress            []*NetworkPlan_Ingress  `protobuf:"bytes,2,rep,name=ingress,proto3" json:"ingress,omitempty"`
	NonLocalManaged    []*NetworkPlan_Ingress  `protobuf:"bytes,3,rep,name=non_local_managed,json=nonLocalManaged,proto3" json:"non_local_managed,omitempty"`
	NonLocalNonManaged []*NetworkPlan_Ingress  `protobuf:"bytes,4,rep,name=non_local_non_managed,json=nonLocalNonManaged,proto3" json:"non_local_non_managed,omitempty"`
	InternalCount      int32                   `protobuf:"varint,5,opt,name=internal_count,json=internalCount,proto3" json:"internal_count,omitempty"`
	LocalHostName      string                  `protobuf:"bytes,6,opt,name=local_host_name,json=localHostName,proto3" json:"local_host_name,omitempty"`
}

func (x *NetworkPlan) Reset() {
	*x = NetworkPlan{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_storage_deploy_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NetworkPlan) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkPlan) ProtoMessage() {}

func (x *NetworkPlan) ProtoReflect() protoreflect.Message {
	mi := &file_schema_storage_deploy_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkPlan.ProtoReflect.Descriptor instead.
func (*NetworkPlan) Descriptor() ([]byte, []int) {
	return file_schema_storage_deploy_proto_rawDescGZIP(), []int{0}
}

func (x *NetworkPlan) GetEndpoint() []*NetworkPlan_Endpoint {
	if x != nil {
		return x.Endpoint
	}
	return nil
}

func (x *NetworkPlan) GetIngress() []*NetworkPlan_Ingress {
	if x != nil {
		return x.Ingress
	}
	return nil
}

func (x *NetworkPlan) GetNonLocalManaged() []*NetworkPlan_Ingress {
	if x != nil {
		return x.NonLocalManaged
	}
	return nil
}

func (x *NetworkPlan) GetNonLocalNonManaged() []*NetworkPlan_Ingress {
	if x != nil {
		return x.NonLocalNonManaged
	}
	return nil
}

func (x *NetworkPlan) GetInternalCount() int32 {
	if x != nil {
		return x.InternalCount
	}
	return 0
}

func (x *NetworkPlan) GetLocalHostName() string {
	if x != nil {
		return x.LocalHostName
	}
	return ""
}

type NetworkPlan_AccessCmd struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// url for http
	// "grpcurl" command line for grpc
	// "curl" command line for http transcoding.
	// "private:" if the service can't be accessed from outside.
	Cmd string `protobuf:"bytes,1,opt,name=cmd,proto3" json:"cmd,omitempty"`
	// Whether it is managed by Namespace.
	IsManaged bool `protobuf:"varint,2,opt,name=is_managed,json=isManaged,proto3" json:"is_managed,omitempty"`
}

func (x *NetworkPlan_AccessCmd) Reset() {
	*x = NetworkPlan_AccessCmd{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_storage_deploy_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NetworkPlan_AccessCmd) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkPlan_AccessCmd) ProtoMessage() {}

func (x *NetworkPlan_AccessCmd) ProtoReflect() protoreflect.Message {
	mi := &file_schema_storage_deploy_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkPlan_AccessCmd.ProtoReflect.Descriptor instead.
func (*NetworkPlan_AccessCmd) Descriptor() ([]byte, []int) {
	return file_schema_storage_deploy_proto_rawDescGZIP(), []int{0, 0}
}

func (x *NetworkPlan_AccessCmd) GetCmd() string {
	if x != nil {
		return x.Cmd
	}
	return ""
}

func (x *NetworkPlan_AccessCmd) GetIsManaged() bool {
	if x != nil {
		return x.IsManaged
	}
	return false
}

type NetworkPlan_Endpoint struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Label           *NetworkPlan_Label       `protobuf:"bytes,1,opt,name=label,proto3" json:"label,omitempty"`
	Focus           bool                     `protobuf:"varint,2,opt,name=focus,proto3" json:"focus,omitempty"`
	Url             string                   `protobuf:"bytes,3,opt,name=url,proto3" json:"url,omitempty"`
	LocalPort       uint32                   `protobuf:"varint,4,opt,name=local_port,json=localPort,proto3" json:"local_port,omitempty"`
	EndpointOwner   string                   `protobuf:"bytes,5,opt,name=endpoint_owner,json=endpointOwner,proto3" json:"endpoint_owner,omitempty"`
	AccessCmd       []*NetworkPlan_AccessCmd `protobuf:"bytes,6,rep,name=access_cmd,json=accessCmd,proto3" json:"access_cmd,omitempty"`
	IsPortForwarded bool                     `protobuf:"varint,7,opt,name=is_port_forwarded,json=isPortForwarded,proto3" json:"is_port_forwarded,omitempty"`
}

func (x *NetworkPlan_Endpoint) Reset() {
	*x = NetworkPlan_Endpoint{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_storage_deploy_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NetworkPlan_Endpoint) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkPlan_Endpoint) ProtoMessage() {}

func (x *NetworkPlan_Endpoint) ProtoReflect() protoreflect.Message {
	mi := &file_schema_storage_deploy_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkPlan_Endpoint.ProtoReflect.Descriptor instead.
func (*NetworkPlan_Endpoint) Descriptor() ([]byte, []int) {
	return file_schema_storage_deploy_proto_rawDescGZIP(), []int{0, 1}
}

func (x *NetworkPlan_Endpoint) GetLabel() *NetworkPlan_Label {
	if x != nil {
		return x.Label
	}
	return nil
}

func (x *NetworkPlan_Endpoint) GetFocus() bool {
	if x != nil {
		return x.Focus
	}
	return false
}

func (x *NetworkPlan_Endpoint) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

func (x *NetworkPlan_Endpoint) GetLocalPort() uint32 {
	if x != nil {
		return x.LocalPort
	}
	return 0
}

func (x *NetworkPlan_Endpoint) GetEndpointOwner() string {
	if x != nil {
		return x.EndpointOwner
	}
	return ""
}

func (x *NetworkPlan_Endpoint) GetAccessCmd() []*NetworkPlan_AccessCmd {
	if x != nil {
		return x.AccessCmd
	}
	return nil
}

func (x *NetworkPlan_Endpoint) GetIsPortForwarded() bool {
	if x != nil {
		return x.IsPortForwarded
	}
	return false
}

type NetworkPlan_Ingress struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Fqdn         string   `protobuf:"bytes,1,opt,name=fqdn,proto3" json:"fqdn,omitempty"`
	Schema       string   `protobuf:"bytes,2,opt,name=schema,proto3" json:"schema,omitempty"`
	PortLabel    string   `protobuf:"bytes,3,opt,name=port_label,json=portLabel,proto3" json:"port_label,omitempty"`
	Command      string   `protobuf:"bytes,4,opt,name=command,proto3" json:"command,omitempty"`
	Comment      string   `protobuf:"bytes,5,opt,name=comment,proto3" json:"comment,omitempty"`
	LocalPort    uint32   `protobuf:"varint,6,opt,name=local_port,json=localPort,proto3" json:"local_port,omitempty"`
	PackageOwner []string `protobuf:"bytes,7,rep,name=package_owner,json=packageOwner,proto3" json:"package_owner,omitempty"`
}

func (x *NetworkPlan_Ingress) Reset() {
	*x = NetworkPlan_Ingress{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_storage_deploy_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NetworkPlan_Ingress) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkPlan_Ingress) ProtoMessage() {}

func (x *NetworkPlan_Ingress) ProtoReflect() protoreflect.Message {
	mi := &file_schema_storage_deploy_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkPlan_Ingress.ProtoReflect.Descriptor instead.
func (*NetworkPlan_Ingress) Descriptor() ([]byte, []int) {
	return file_schema_storage_deploy_proto_rawDescGZIP(), []int{0, 2}
}

func (x *NetworkPlan_Ingress) GetFqdn() string {
	if x != nil {
		return x.Fqdn
	}
	return ""
}

func (x *NetworkPlan_Ingress) GetSchema() string {
	if x != nil {
		return x.Schema
	}
	return ""
}

func (x *NetworkPlan_Ingress) GetPortLabel() string {
	if x != nil {
		return x.PortLabel
	}
	return ""
}

func (x *NetworkPlan_Ingress) GetCommand() string {
	if x != nil {
		return x.Command
	}
	return ""
}

func (x *NetworkPlan_Ingress) GetComment() string {
	if x != nil {
		return x.Comment
	}
	return ""
}

func (x *NetworkPlan_Ingress) GetLocalPort() uint32 {
	if x != nil {
		return x.LocalPort
	}
	return 0
}

func (x *NetworkPlan_Ingress) GetPackageOwner() []string {
	if x != nil {
		return x.PackageOwner
	}
	return nil
}

type NetworkPlan_Label struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Label        string `protobuf:"bytes,1,opt,name=label,proto3" json:"label,omitempty"`
	ServiceProto string `protobuf:"bytes,2,opt,name=service_proto,json=serviceProto,proto3" json:"service_proto,omitempty"`
}

func (x *NetworkPlan_Label) Reset() {
	*x = NetworkPlan_Label{}
	if protoimpl.UnsafeEnabled {
		mi := &file_schema_storage_deploy_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NetworkPlan_Label) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NetworkPlan_Label) ProtoMessage() {}

func (x *NetworkPlan_Label) ProtoReflect() protoreflect.Message {
	mi := &file_schema_storage_deploy_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NetworkPlan_Label.ProtoReflect.Descriptor instead.
func (*NetworkPlan_Label) Descriptor() ([]byte, []int) {
	return file_schema_storage_deploy_proto_rawDescGZIP(), []int{0, 3}
}

func (x *NetworkPlan_Label) GetLabel() string {
	if x != nil {
		return x.Label
	}
	return ""
}

func (x *NetworkPlan_Label) GetServiceProto() string {
	if x != nil {
		return x.ServiceProto
	}
	return ""
}

var File_schema_storage_deploy_proto protoreflect.FileDescriptor

var file_schema_storage_deploy_proto_rawDesc = []byte{
	0x0a, 0x1b, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x73, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65,
	0x2f, 0x64, 0x65, 0x70, 0x6c, 0x6f, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x19, 0x66,
	0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x2e, 0x73, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x22, 0xbf, 0x08, 0x0a, 0x0b, 0x4e, 0x65, 0x74,
	0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c, 0x61, 0x6e, 0x12, 0x4b, 0x0a, 0x08, 0x65, 0x6e, 0x64, 0x70,
	0x6f, 0x69, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2f, 0x2e, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x73,
	0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c,
	0x61, 0x6e, 0x2e, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x52, 0x08, 0x65, 0x6e, 0x64,
	0x70, 0x6f, 0x69, 0x6e, 0x74, 0x12, 0x48, 0x0a, 0x07, 0x69, 0x6e, 0x67, 0x72, 0x65, 0x73, 0x73,
	0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x73, 0x74, 0x6f, 0x72, 0x61,
	0x67, 0x65, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c, 0x61, 0x6e, 0x2e, 0x49,
	0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x52, 0x07, 0x69, 0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x12,
	0x5a, 0x0a, 0x11, 0x6e, 0x6f, 0x6e, 0x5f, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x6d, 0x61, 0x6e,
	0x61, 0x67, 0x65, 0x64, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x73,
	0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c,
	0x61, 0x6e, 0x2e, 0x49, 0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x52, 0x0f, 0x6e, 0x6f, 0x6e, 0x4c,
	0x6f, 0x63, 0x61, 0x6c, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x64, 0x12, 0x61, 0x0a, 0x15, 0x6e,
	0x6f, 0x6e, 0x5f, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x6e, 0x6f, 0x6e, 0x5f, 0x6d, 0x61, 0x6e,
	0x61, 0x67, 0x65, 0x64, 0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2e, 0x2e, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x73,
	0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c,
	0x61, 0x6e, 0x2e, 0x49, 0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x52, 0x12, 0x6e, 0x6f, 0x6e, 0x4c,
	0x6f, 0x63, 0x61, 0x6c, 0x4e, 0x6f, 0x6e, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x64, 0x12, 0x25,
	0x0a, 0x0e, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x5f, 0x63, 0x6f, 0x75, 0x6e, 0x74,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0d, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c,
	0x43, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x26, 0x0a, 0x0f, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x68,
	0x6f, 0x73, 0x74, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d,
	0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x48, 0x6f, 0x73, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x1a, 0x3c, 0x0a,
	0x09, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73, 0x43, 0x6d, 0x64, 0x12, 0x10, 0x0a, 0x03, 0x63, 0x6d,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x63, 0x6d, 0x64, 0x12, 0x1d, 0x0a, 0x0a,
	0x69, 0x73, 0x5f, 0x6d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x09, 0x69, 0x73, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x64, 0x1a, 0xb9, 0x02, 0x0a, 0x08,
	0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x12, 0x42, 0x0a, 0x05, 0x6c, 0x61, 0x62, 0x65,
	0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x2c, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x73, 0x74, 0x6f, 0x72,
	0x61, 0x67, 0x65, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x50, 0x6c, 0x61, 0x6e, 0x2e,
	0x4c, 0x61, 0x62, 0x65, 0x6c, 0x52, 0x05, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x14, 0x0a, 0x05,
	0x66, 0x6f, 0x63, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x66, 0x6f, 0x63,
	0x75, 0x73, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x03, 0x75, 0x72, 0x6c, 0x12, 0x1d, 0x0a, 0x0a, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x70, 0x6f,
	0x72, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x09, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x50,
	0x6f, 0x72, 0x74, 0x12, 0x25, 0x0a, 0x0e, 0x65, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x5f,
	0x6f, 0x77, 0x6e, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x65, 0x6e, 0x64,
	0x70, 0x6f, 0x69, 0x6e, 0x74, 0x4f, 0x77, 0x6e, 0x65, 0x72, 0x12, 0x4f, 0x0a, 0x0a, 0x61, 0x63,
	0x63, 0x65, 0x73, 0x73, 0x5f, 0x63, 0x6d, 0x64, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x30,
	0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65,
	0x6d, 0x61, 0x2e, 0x73, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x2e, 0x4e, 0x65, 0x74, 0x77, 0x6f,
	0x72, 0x6b, 0x50, 0x6c, 0x61, 0x6e, 0x2e, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73, 0x43, 0x6d, 0x64,
	0x52, 0x09, 0x61, 0x63, 0x63, 0x65, 0x73, 0x73, 0x43, 0x6d, 0x64, 0x12, 0x2a, 0x0a, 0x11, 0x69,
	0x73, 0x5f, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x66, 0x6f, 0x72, 0x77, 0x61, 0x72, 0x64, 0x65, 0x64,
	0x18, 0x07, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0f, 0x69, 0x73, 0x50, 0x6f, 0x72, 0x74, 0x46, 0x6f,
	0x72, 0x77, 0x61, 0x72, 0x64, 0x65, 0x64, 0x1a, 0xcc, 0x01, 0x0a, 0x07, 0x49, 0x6e, 0x67, 0x72,
	0x65, 0x73, 0x73, 0x12, 0x12, 0x0a, 0x04, 0x66, 0x71, 0x64, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x66, 0x71, 0x64, 0x6e, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x63, 0x68, 0x65, 0x6d,
	0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x12,
	0x1d, 0x0a, 0x0a, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x09, 0x70, 0x6f, 0x72, 0x74, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x18,
	0x0a, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x6f, 0x6d, 0x6d,
	0x65, 0x6e, 0x74, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x6f, 0x6d, 0x6d, 0x65,
	0x6e, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x70, 0x6f, 0x72, 0x74,
	0x18, 0x06, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x09, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x50, 0x6f, 0x72,
	0x74, 0x12, 0x23, 0x0a, 0x0d, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x6f, 0x77, 0x6e,
	0x65, 0x72, 0x18, 0x07, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0c, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67,
	0x65, 0x4f, 0x77, 0x6e, 0x65, 0x72, 0x1a, 0x42, 0x0a, 0x05, 0x4c, 0x61, 0x62, 0x65, 0x6c, 0x12,
	0x14, 0x0a, 0x05, 0x6c, 0x61, 0x62, 0x65, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05,
	0x6c, 0x61, 0x62, 0x65, 0x6c, 0x12, 0x23, 0x0a, 0x0d, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65,
	0x5f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x42, 0x2d, 0x5a, 0x2b, 0x6e, 0x61,
	0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f,
	0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x6d,
	0x61, 0x2f, 0x73, 0x74, 0x6f, 0x72, 0x61, 0x67, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_schema_storage_deploy_proto_rawDescOnce sync.Once
	file_schema_storage_deploy_proto_rawDescData = file_schema_storage_deploy_proto_rawDesc
)

func file_schema_storage_deploy_proto_rawDescGZIP() []byte {
	file_schema_storage_deploy_proto_rawDescOnce.Do(func() {
		file_schema_storage_deploy_proto_rawDescData = protoimpl.X.CompressGZIP(file_schema_storage_deploy_proto_rawDescData)
	})
	return file_schema_storage_deploy_proto_rawDescData
}

var file_schema_storage_deploy_proto_msgTypes = make([]protoimpl.MessageInfo, 5)
var file_schema_storage_deploy_proto_goTypes = []interface{}{
	(*NetworkPlan)(nil),           // 0: foundation.schema.storage.NetworkPlan
	(*NetworkPlan_AccessCmd)(nil), // 1: foundation.schema.storage.NetworkPlan.AccessCmd
	(*NetworkPlan_Endpoint)(nil),  // 2: foundation.schema.storage.NetworkPlan.Endpoint
	(*NetworkPlan_Ingress)(nil),   // 3: foundation.schema.storage.NetworkPlan.Ingress
	(*NetworkPlan_Label)(nil),     // 4: foundation.schema.storage.NetworkPlan.Label
}
var file_schema_storage_deploy_proto_depIdxs = []int32{
	2, // 0: foundation.schema.storage.NetworkPlan.endpoint:type_name -> foundation.schema.storage.NetworkPlan.Endpoint
	3, // 1: foundation.schema.storage.NetworkPlan.ingress:type_name -> foundation.schema.storage.NetworkPlan.Ingress
	3, // 2: foundation.schema.storage.NetworkPlan.non_local_managed:type_name -> foundation.schema.storage.NetworkPlan.Ingress
	3, // 3: foundation.schema.storage.NetworkPlan.non_local_non_managed:type_name -> foundation.schema.storage.NetworkPlan.Ingress
	4, // 4: foundation.schema.storage.NetworkPlan.Endpoint.label:type_name -> foundation.schema.storage.NetworkPlan.Label
	1, // 5: foundation.schema.storage.NetworkPlan.Endpoint.access_cmd:type_name -> foundation.schema.storage.NetworkPlan.AccessCmd
	6, // [6:6] is the sub-list for method output_type
	6, // [6:6] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_schema_storage_deploy_proto_init() }
func file_schema_storage_deploy_proto_init() {
	if File_schema_storage_deploy_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_schema_storage_deploy_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NetworkPlan); i {
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
		file_schema_storage_deploy_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NetworkPlan_AccessCmd); i {
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
		file_schema_storage_deploy_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NetworkPlan_Endpoint); i {
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
		file_schema_storage_deploy_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NetworkPlan_Ingress); i {
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
		file_schema_storage_deploy_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NetworkPlan_Label); i {
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
			RawDescriptor: file_schema_storage_deploy_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   5,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_schema_storage_deploy_proto_goTypes,
		DependencyIndexes: file_schema_storage_deploy_proto_depIdxs,
		MessageInfos:      file_schema_storage_deploy_proto_msgTypes,
	}.Build()
	File_schema_storage_deploy_proto = out.File
	file_schema_storage_deploy_proto_rawDesc = nil
	file_schema_storage_deploy_proto_goTypes = nil
	file_schema_storage_deploy_proto_depIdxs = nil
}
