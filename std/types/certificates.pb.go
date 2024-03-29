// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: std/types/certificates.proto

package types

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

type TLSCertificateSpec struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Organization []string `protobuf:"bytes,1,rep,name=organization,proto3" json:"organization,omitempty"`
	Description  string   `protobuf:"bytes,2,opt,name=description,proto3" json:"description,omitempty"`
	CommonName   string   `protobuf:"bytes,3,opt,name=common_name,json=commonName,proto3" json:"common_name,omitempty"`
	DnsName      []string `protobuf:"bytes,4,rep,name=dns_name,json=dnsName,proto3" json:"dns_name,omitempty"`
}

func (x *TLSCertificateSpec) Reset() {
	*x = TLSCertificateSpec{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_types_certificates_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TLSCertificateSpec) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TLSCertificateSpec) ProtoMessage() {}

func (x *TLSCertificateSpec) ProtoReflect() protoreflect.Message {
	mi := &file_std_types_certificates_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TLSCertificateSpec.ProtoReflect.Descriptor instead.
func (*TLSCertificateSpec) Descriptor() ([]byte, []int) {
	return file_std_types_certificates_proto_rawDescGZIP(), []int{0}
}

func (x *TLSCertificateSpec) GetOrganization() []string {
	if x != nil {
		return x.Organization
	}
	return nil
}

func (x *TLSCertificateSpec) GetDescription() string {
	if x != nil {
		return x.Description
	}
	return ""
}

func (x *TLSCertificateSpec) GetCommonName() string {
	if x != nil {
		return x.CommonName
	}
	return ""
}

func (x *TLSCertificateSpec) GetDnsName() []string {
	if x != nil {
		return x.DnsName
	}
	return nil
}

type CertificateChain struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CA     *schema.Certificate `protobuf:"bytes,1,opt,name=CA,proto3" json:"CA,omitempty"`
	Server *schema.Certificate `protobuf:"bytes,2,opt,name=Server,proto3" json:"Server,omitempty"`
}

func (x *CertificateChain) Reset() {
	*x = CertificateChain{}
	if protoimpl.UnsafeEnabled {
		mi := &file_std_types_certificates_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CertificateChain) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CertificateChain) ProtoMessage() {}

func (x *CertificateChain) ProtoReflect() protoreflect.Message {
	mi := &file_std_types_certificates_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CertificateChain.ProtoReflect.Descriptor instead.
func (*CertificateChain) Descriptor() ([]byte, []int) {
	return file_std_types_certificates_proto_rawDescGZIP(), []int{1}
}

func (x *CertificateChain) GetCA() *schema.Certificate {
	if x != nil {
		return x.CA
	}
	return nil
}

func (x *CertificateChain) GetServer() *schema.Certificate {
	if x != nil {
		return x.Server
	}
	return nil
}

var File_std_types_certificates_proto protoreflect.FileDescriptor

var file_std_types_certificates_proto_rawDesc = []byte{
	0x0a, 0x1c, 0x73, 0x74, 0x64, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2f, 0x63, 0x65, 0x72, 0x74,
	0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x14,
	0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x74, 0x64, 0x2e, 0x74,
	0x79, 0x70, 0x65, 0x73, 0x1a, 0x13, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x64, 0x6f, 0x6d,
	0x61, 0x69, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x96, 0x01, 0x0a, 0x12, 0x54, 0x4c,
	0x53, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x53, 0x70, 0x65, 0x63,
	0x12, 0x22, 0x0a, 0x0c, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69, 0x7a, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x18, 0x01, 0x20, 0x03, 0x28, 0x09, 0x52, 0x0c, 0x6f, 0x72, 0x67, 0x61, 0x6e, 0x69, 0x7a, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x12, 0x20, 0x0a, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x64, 0x65, 0x73, 0x63, 0x72,
	0x69, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x12, 0x1f, 0x0a, 0x0b, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e,
	0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x63, 0x6f, 0x6d,
	0x6d, 0x6f, 0x6e, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x64, 0x6e, 0x73, 0x5f, 0x6e,
	0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x64, 0x6e, 0x73, 0x4e, 0x61,
	0x6d, 0x65, 0x22, 0x7a, 0x0a, 0x10, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74,
	0x65, 0x43, 0x68, 0x61, 0x69, 0x6e, 0x12, 0x2e, 0x0a, 0x02, 0x43, 0x41, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e,
	0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x43, 0x65, 0x72, 0x74, 0x69, 0x66, 0x69, 0x63, 0x61,
	0x74, 0x65, 0x52, 0x02, 0x43, 0x41, 0x12, 0x36, 0x0a, 0x06, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x43, 0x65, 0x72, 0x74, 0x69,
	0x66, 0x69, 0x63, 0x61, 0x74, 0x65, 0x52, 0x06, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x42, 0x28,
	0x5a, 0x26, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e,
	0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x73,
	0x74, 0x64, 0x2f, 0x74, 0x79, 0x70, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_std_types_certificates_proto_rawDescOnce sync.Once
	file_std_types_certificates_proto_rawDescData = file_std_types_certificates_proto_rawDesc
)

func file_std_types_certificates_proto_rawDescGZIP() []byte {
	file_std_types_certificates_proto_rawDescOnce.Do(func() {
		file_std_types_certificates_proto_rawDescData = protoimpl.X.CompressGZIP(file_std_types_certificates_proto_rawDescData)
	})
	return file_std_types_certificates_proto_rawDescData
}

var file_std_types_certificates_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_std_types_certificates_proto_goTypes = []interface{}{
	(*TLSCertificateSpec)(nil), // 0: foundation.std.types.TLSCertificateSpec
	(*CertificateChain)(nil),   // 1: foundation.std.types.CertificateChain
	(*schema.Certificate)(nil), // 2: foundation.schema.Certificate
}
var file_std_types_certificates_proto_depIdxs = []int32{
	2, // 0: foundation.std.types.CertificateChain.CA:type_name -> foundation.schema.Certificate
	2, // 1: foundation.std.types.CertificateChain.Server:type_name -> foundation.schema.Certificate
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_std_types_certificates_proto_init() }
func file_std_types_certificates_proto_init() {
	if File_std_types_certificates_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_std_types_certificates_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TLSCertificateSpec); i {
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
		file_std_types_certificates_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CertificateChain); i {
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
			RawDescriptor: file_std_types_certificates_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_std_types_certificates_proto_goTypes,
		DependencyIndexes: file_std_types_certificates_proto_depIdxs,
		MessageInfos:      file_std_types_certificates_proto_msgTypes,
	}.Build()
	File_std_types_certificates_proto = out.File
	file_std_types_certificates_proto_rawDesc = nil
	file_std_types_certificates_proto_goTypes = nil
	file_std_types_certificates_proto_depIdxs = nil
}
