// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: universe/vault/types.proto

package vault

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

type AppRole struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name       string         `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	AuthMethod string         `protobuf:"bytes,2,opt,name=auth_method,json=authMethod,proto3" json:"auth_method,omitempty"`
	Provider   *VaultProvider `protobuf:"bytes,3,opt,name=provider,proto3" json:"provider,omitempty"`
}

func (x *AppRole) Reset() {
	*x = AppRole{}
	if protoimpl.UnsafeEnabled {
		mi := &file_universe_vault_types_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AppRole) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AppRole) ProtoMessage() {}

func (x *AppRole) ProtoReflect() protoreflect.Message {
	mi := &file_universe_vault_types_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AppRole.ProtoReflect.Descriptor instead.
func (*AppRole) Descriptor() ([]byte, []int) {
	return file_universe_vault_types_proto_rawDescGZIP(), []int{0}
}

func (x *AppRole) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *AppRole) GetAuthMethod() string {
	if x != nil {
		return x.AuthMethod
	}
	return ""
}

func (x *AppRole) GetProvider() *VaultProvider {
	if x != nil {
		return x.Provider
	}
	return nil
}

type Certificate struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BaseDomain string         `protobuf:"bytes,1,opt,name=base_domain,json=baseDomain,proto3" json:"base_domain,omitempty"`
	Issuer     string         `protobuf:"bytes,3,opt,name=issuer,proto3" json:"issuer,omitempty"`
	Provider   *VaultProvider `protobuf:"bytes,4,opt,name=provider,proto3" json:"provider,omitempty"`
}

func (x *Certificate) Reset() {
	*x = Certificate{}
	if protoimpl.UnsafeEnabled {
		mi := &file_universe_vault_types_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Certificate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Certificate) ProtoMessage() {}

func (x *Certificate) ProtoReflect() protoreflect.Message {
	mi := &file_universe_vault_types_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Certificate.ProtoReflect.Descriptor instead.
func (*Certificate) Descriptor() ([]byte, []int) {
	return file_universe_vault_types_proto_rawDescGZIP(), []int{1}
}

func (x *Certificate) GetBaseDomain() string {
	if x != nil {
		return x.BaseDomain
	}
	return ""
}

func (x *Certificate) GetIssuer() string {
	if x != nil {
		return x.Issuer
	}
	return ""
}

func (x *Certificate) GetProvider() *VaultProvider {
	if x != nil {
		return x.Provider
	}
	return nil
}

type VaultProvider struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Address    string `protobuf:"bytes,1,opt,name=address,proto3" json:"address,omitempty"`
	Namespace  string `protobuf:"bytes,2,opt,name=namespace,proto3" json:"namespace,omitempty"`
	AuthMethod string `protobuf:"bytes,4,opt,name=auth_method,json=authMethod,proto3" json:"auth_method,omitempty"`
}

func (x *VaultProvider) Reset() {
	*x = VaultProvider{}
	if protoimpl.UnsafeEnabled {
		mi := &file_universe_vault_types_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *VaultProvider) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*VaultProvider) ProtoMessage() {}

func (x *VaultProvider) ProtoReflect() protoreflect.Message {
	mi := &file_universe_vault_types_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use VaultProvider.ProtoReflect.Descriptor instead.
func (*VaultProvider) Descriptor() ([]byte, []int) {
	return file_universe_vault_types_proto_rawDescGZIP(), []int{2}
}

func (x *VaultProvider) GetAddress() string {
	if x != nil {
		return x.Address
	}
	return ""
}

func (x *VaultProvider) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *VaultProvider) GetAuthMethod() string {
	if x != nil {
		return x.AuthMethod
	}
	return ""
}

var File_universe_vault_types_proto protoreflect.FileDescriptor

var file_universe_vault_types_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2f, 0x76, 0x61, 0x75, 0x6c, 0x74,
	0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x19, 0x66, 0x6f,
	0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73,
	0x65, 0x2e, 0x76, 0x61, 0x75, 0x6c, 0x74, 0x22, 0x84, 0x01, 0x0a, 0x07, 0x41, 0x70, 0x70, 0x52,
	0x6f, 0x6c, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x1f, 0x0a, 0x0b, 0x61, 0x75, 0x74, 0x68, 0x5f,
	0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x61, 0x75,
	0x74, 0x68, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x12, 0x44, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x76,
	0x69, 0x64, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x28, 0x2e, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65,
	0x2e, 0x76, 0x61, 0x75, 0x6c, 0x74, 0x2e, 0x56, 0x61, 0x75, 0x6c, 0x74, 0x50, 0x72, 0x6f, 0x76,
	0x69, 0x64, 0x65, 0x72, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x22, 0x92,
	0x01, 0x0a, 0x0b, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x12, 0x1f,
	0x0a, 0x0b, 0x62, 0x61, 0x73, 0x65, 0x5f, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0a, 0x62, 0x61, 0x73, 0x65, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x12,
	0x16, 0x0a, 0x06, 0x69, 0x73, 0x73, 0x75, 0x65, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x69, 0x73, 0x73, 0x75, 0x65, 0x72, 0x12, 0x44, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x76, 0x69,
	0x64, 0x65, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x28, 0x2e, 0x66, 0x6f, 0x75, 0x6e,
	0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2e,
	0x76, 0x61, 0x75, 0x6c, 0x74, 0x2e, 0x56, 0x61, 0x75, 0x6c, 0x74, 0x50, 0x72, 0x6f, 0x76, 0x69,
	0x64, 0x65, 0x72, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x4a, 0x04, 0x08,
	0x02, 0x10, 0x03, 0x22, 0x68, 0x0a, 0x0d, 0x56, 0x61, 0x75, 0x6c, 0x74, 0x50, 0x72, 0x6f, 0x76,
	0x69, 0x64, 0x65, 0x72, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x64, 0x64, 0x72, 0x65, 0x73, 0x73, 0x12, 0x1c,
	0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x1f, 0x0a, 0x0b,
	0x61, 0x75, 0x74, 0x68, 0x5f, 0x6d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0a, 0x61, 0x75, 0x74, 0x68, 0x4d, 0x65, 0x74, 0x68, 0x6f, 0x64, 0x42, 0x2d, 0x5a,
	0x2b, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64,
	0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x75, 0x6e,
	0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2f, 0x76, 0x61, 0x75, 0x6c, 0x74, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_universe_vault_types_proto_rawDescOnce sync.Once
	file_universe_vault_types_proto_rawDescData = file_universe_vault_types_proto_rawDesc
)

func file_universe_vault_types_proto_rawDescGZIP() []byte {
	file_universe_vault_types_proto_rawDescOnce.Do(func() {
		file_universe_vault_types_proto_rawDescData = protoimpl.X.CompressGZIP(file_universe_vault_types_proto_rawDescData)
	})
	return file_universe_vault_types_proto_rawDescData
}

var file_universe_vault_types_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_universe_vault_types_proto_goTypes = []interface{}{
	(*AppRole)(nil),       // 0: foundation.universe.vault.AppRole
	(*Certificate)(nil),   // 1: foundation.universe.vault.Certificate
	(*VaultProvider)(nil), // 2: foundation.universe.vault.VaultProvider
}
var file_universe_vault_types_proto_depIdxs = []int32{
	2, // 0: foundation.universe.vault.AppRole.provider:type_name -> foundation.universe.vault.VaultProvider
	2, // 1: foundation.universe.vault.Certificate.provider:type_name -> foundation.universe.vault.VaultProvider
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_universe_vault_types_proto_init() }
func file_universe_vault_types_proto_init() {
	if File_universe_vault_types_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_universe_vault_types_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AppRole); i {
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
		file_universe_vault_types_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Certificate); i {
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
		file_universe_vault_types_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*VaultProvider); i {
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
			RawDescriptor: file_universe_vault_types_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_universe_vault_types_proto_goTypes,
		DependencyIndexes: file_universe_vault_types_proto_depIdxs,
		MessageInfos:      file_universe_vault_types_proto_msgTypes,
	}.Build()
	File_universe_vault_types_proto = out.File
	file_universe_vault_types_proto_rawDesc = nil
	file_universe_vault_types_proto_goTypes = nil
	file_universe_vault_types_proto_depIdxs = nil
}
