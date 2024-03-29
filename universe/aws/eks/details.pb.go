// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: universe/aws/eks/details.proto

package eks

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

// Next ID: 7
type EKSCluster struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name            string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Arn             string `protobuf:"bytes,2,opt,name=arn,proto3" json:"arn,omitempty"`
	OidcIssuer      string `protobuf:"bytes,3,opt,name=oidc_issuer,json=oidcIssuer,proto3" json:"oidc_issuer,omitempty"`
	VpcId           string `protobuf:"bytes,4,opt,name=vpc_id,json=vpcId,proto3" json:"vpc_id,omitempty"`
	SecurityGroupId string `protobuf:"bytes,6,opt,name=security_group_id,json=securityGroupId,proto3" json:"security_group_id,omitempty"`
	// Whether the "oidc_issuer" is a registered OIDC provider.
	// When false but needed, we ask the user to register the OIDC provider similar to this:
	//   https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html
	HasOidcProvider bool `protobuf:"varint,5,opt,name=has_oidc_provider,json=hasOidcProvider,proto3" json:"has_oidc_provider,omitempty"`
}

func (x *EKSCluster) Reset() {
	*x = EKSCluster{}
	if protoimpl.UnsafeEnabled {
		mi := &file_universe_aws_eks_details_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EKSCluster) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EKSCluster) ProtoMessage() {}

func (x *EKSCluster) ProtoReflect() protoreflect.Message {
	mi := &file_universe_aws_eks_details_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EKSCluster.ProtoReflect.Descriptor instead.
func (*EKSCluster) Descriptor() ([]byte, []int) {
	return file_universe_aws_eks_details_proto_rawDescGZIP(), []int{0}
}

func (x *EKSCluster) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *EKSCluster) GetArn() string {
	if x != nil {
		return x.Arn
	}
	return ""
}

func (x *EKSCluster) GetOidcIssuer() string {
	if x != nil {
		return x.OidcIssuer
	}
	return ""
}

func (x *EKSCluster) GetVpcId() string {
	if x != nil {
		return x.VpcId
	}
	return ""
}

func (x *EKSCluster) GetSecurityGroupId() string {
	if x != nil {
		return x.SecurityGroupId
	}
	return ""
}

func (x *EKSCluster) GetHasOidcProvider() bool {
	if x != nil {
		return x.HasOidcProvider
	}
	return false
}

type EKSServerDetails struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ComputedIamRoleName string `protobuf:"bytes,1,opt,name=computed_iam_role_name,json=computedIamRoleName,proto3" json:"computed_iam_role_name,omitempty"` // This role is not instantiated by default.
}

func (x *EKSServerDetails) Reset() {
	*x = EKSServerDetails{}
	if protoimpl.UnsafeEnabled {
		mi := &file_universe_aws_eks_details_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EKSServerDetails) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EKSServerDetails) ProtoMessage() {}

func (x *EKSServerDetails) ProtoReflect() protoreflect.Message {
	mi := &file_universe_aws_eks_details_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EKSServerDetails.ProtoReflect.Descriptor instead.
func (*EKSServerDetails) Descriptor() ([]byte, []int) {
	return file_universe_aws_eks_details_proto_rawDescGZIP(), []int{1}
}

func (x *EKSServerDetails) GetComputedIamRoleName() string {
	if x != nil {
		return x.ComputedIamRoleName
	}
	return ""
}

var File_universe_aws_eks_details_proto protoreflect.FileDescriptor

var file_universe_aws_eks_details_proto_rawDesc = []byte{
	0x0a, 0x1e, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2f, 0x61, 0x77, 0x73, 0x2f, 0x65,
	0x6b, 0x73, 0x2f, 0x64, 0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x1b, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x75, 0x6e, 0x69,
	0x76, 0x65, 0x72, 0x73, 0x65, 0x2e, 0x61, 0x77, 0x73, 0x2e, 0x65, 0x6b, 0x73, 0x22, 0xc2, 0x01,
	0x0a, 0x0a, 0x45, 0x4b, 0x53, 0x43, 0x6c, 0x75, 0x73, 0x74, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x12, 0x10, 0x0a, 0x03, 0x61, 0x72, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x61,
	0x72, 0x6e, 0x12, 0x1f, 0x0a, 0x0b, 0x6f, 0x69, 0x64, 0x63, 0x5f, 0x69, 0x73, 0x73, 0x75, 0x65,
	0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x6f, 0x69, 0x64, 0x63, 0x49, 0x73, 0x73,
	0x75, 0x65, 0x72, 0x12, 0x15, 0x0a, 0x06, 0x76, 0x70, 0x63, 0x5f, 0x69, 0x64, 0x18, 0x04, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x70, 0x63, 0x49, 0x64, 0x12, 0x2a, 0x0a, 0x11, 0x73, 0x65,
	0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x5f, 0x67, 0x72, 0x6f, 0x75, 0x70, 0x5f, 0x69, 0x64, 0x18,
	0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x73, 0x65, 0x63, 0x75, 0x72, 0x69, 0x74, 0x79, 0x47,
	0x72, 0x6f, 0x75, 0x70, 0x49, 0x64, 0x12, 0x2a, 0x0a, 0x11, 0x68, 0x61, 0x73, 0x5f, 0x6f, 0x69,
	0x64, 0x63, 0x5f, 0x70, 0x72, 0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x0f, 0x68, 0x61, 0x73, 0x4f, 0x69, 0x64, 0x63, 0x50, 0x72, 0x6f, 0x76, 0x69, 0x64,
	0x65, 0x72, 0x22, 0x47, 0x0a, 0x10, 0x45, 0x4b, 0x53, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x44,
	0x65, 0x74, 0x61, 0x69, 0x6c, 0x73, 0x12, 0x33, 0x0a, 0x16, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74,
	0x65, 0x64, 0x5f, 0x69, 0x61, 0x6d, 0x5f, 0x72, 0x6f, 0x6c, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x13, 0x63, 0x6f, 0x6d, 0x70, 0x75, 0x74, 0x65, 0x64,
	0x49, 0x61, 0x6d, 0x52, 0x6f, 0x6c, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x42, 0x2f, 0x5a, 0x2d, 0x6e,
	0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c, 0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76,
	0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x75, 0x6e, 0x69, 0x76,
	0x65, 0x72, 0x73, 0x65, 0x2f, 0x61, 0x77, 0x73, 0x2f, 0x65, 0x6b, 0x73, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_universe_aws_eks_details_proto_rawDescOnce sync.Once
	file_universe_aws_eks_details_proto_rawDescData = file_universe_aws_eks_details_proto_rawDesc
)

func file_universe_aws_eks_details_proto_rawDescGZIP() []byte {
	file_universe_aws_eks_details_proto_rawDescOnce.Do(func() {
		file_universe_aws_eks_details_proto_rawDescData = protoimpl.X.CompressGZIP(file_universe_aws_eks_details_proto_rawDescData)
	})
	return file_universe_aws_eks_details_proto_rawDescData
}

var file_universe_aws_eks_details_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_universe_aws_eks_details_proto_goTypes = []interface{}{
	(*EKSCluster)(nil),       // 0: foundation.universe.aws.eks.EKSCluster
	(*EKSServerDetails)(nil), // 1: foundation.universe.aws.eks.EKSServerDetails
}
var file_universe_aws_eks_details_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_universe_aws_eks_details_proto_init() }
func file_universe_aws_eks_details_proto_init() {
	if File_universe_aws_eks_details_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_universe_aws_eks_details_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EKSCluster); i {
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
		file_universe_aws_eks_details_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EKSServerDetails); i {
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
			RawDescriptor: file_universe_aws_eks_details_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_universe_aws_eks_details_proto_goTypes,
		DependencyIndexes: file_universe_aws_eks_details_proto_depIdxs,
		MessageInfos:      file_universe_aws_eks_details_proto_msgTypes,
	}.Build()
	File_universe_aws_eks_details_proto = out.File
	file_universe_aws_eks_details_proto_rawDesc = nil
	file_universe_aws_eks_details_proto_goTypes = nil
	file_universe_aws_eks_details_proto_depIdxs = nil
}
