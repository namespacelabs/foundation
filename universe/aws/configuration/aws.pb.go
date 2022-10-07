// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: universe/aws/configuration/aws.proto

package configuration

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
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

	Profile                string       `protobuf:"bytes,1,opt,name=profile,proto3" json:"profile,omitempty"`
	UseInjectedWebIdentity bool         `protobuf:"varint,2,opt,name=use_injected_web_identity,json=useInjectedWebIdentity,proto3" json:"use_injected_web_identity,omitempty"`
	AssumeRoleArn          string       `protobuf:"bytes,3,opt,name=assume_role_arn,json=assumeRoleArn,proto3" json:"assume_role_arn,omitempty"`
	Region                 string       `protobuf:"bytes,4,opt,name=region,proto3" json:"region,omitempty"`
	Static                 *Credentials `protobuf:"bytes,5,opt,name=static,proto3" json:"static,omitempty"`
}

func (x *Configuration) Reset() {
	*x = Configuration{}
	if protoimpl.UnsafeEnabled {
		mi := &file_universe_aws_configuration_aws_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Configuration) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Configuration) ProtoMessage() {}

func (x *Configuration) ProtoReflect() protoreflect.Message {
	mi := &file_universe_aws_configuration_aws_proto_msgTypes[0]
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
	return file_universe_aws_configuration_aws_proto_rawDescGZIP(), []int{0}
}

func (x *Configuration) GetProfile() string {
	if x != nil {
		return x.Profile
	}
	return ""
}

func (x *Configuration) GetUseInjectedWebIdentity() bool {
	if x != nil {
		return x.UseInjectedWebIdentity
	}
	return false
}

func (x *Configuration) GetAssumeRoleArn() string {
	if x != nil {
		return x.AssumeRoleArn
	}
	return ""
}

func (x *Configuration) GetRegion() string {
	if x != nil {
		return x.Region
	}
	return ""
}

func (x *Configuration) GetStatic() *Credentials {
	if x != nil {
		return x.Static
	}
	return nil
}

type Credentials struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// The access key ID that identifies the temporary security credentials.
	AccessKeyId string `protobuf:"bytes,1,opt,name=access_key_id,json=accessKeyId,proto3" json:"access_key_id,omitempty"`
	// The date on which the current credentials expire.
	Expiration *timestamppb.Timestamp `protobuf:"bytes,2,opt,name=expiration,proto3" json:"expiration,omitempty"`
	// The secret access key that can be used to sign requests.
	SecretAccessKey string `protobuf:"bytes,3,opt,name=secret_access_key,json=secretAccessKey,proto3" json:"secret_access_key,omitempty"`
	// The token that users must pass to the service API to use the temporary
	// credentials.
	SessionToken string `protobuf:"bytes,4,opt,name=session_token,json=sessionToken,proto3" json:"session_token,omitempty"`
}

func (x *Credentials) Reset() {
	*x = Credentials{}
	if protoimpl.UnsafeEnabled {
		mi := &file_universe_aws_configuration_aws_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Credentials) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Credentials) ProtoMessage() {}

func (x *Credentials) ProtoReflect() protoreflect.Message {
	mi := &file_universe_aws_configuration_aws_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Credentials.ProtoReflect.Descriptor instead.
func (*Credentials) Descriptor() ([]byte, []int) {
	return file_universe_aws_configuration_aws_proto_rawDescGZIP(), []int{1}
}

func (x *Credentials) GetAccessKeyId() string {
	if x != nil {
		return x.AccessKeyId
	}
	return ""
}

func (x *Credentials) GetExpiration() *timestamppb.Timestamp {
	if x != nil {
		return x.Expiration
	}
	return nil
}

func (x *Credentials) GetSecretAccessKey() string {
	if x != nil {
		return x.SecretAccessKey
	}
	return ""
}

func (x *Credentials) GetSessionToken() string {
	if x != nil {
		return x.SessionToken
	}
	return ""
}

var File_universe_aws_configuration_aws_proto protoreflect.FileDescriptor

var file_universe_aws_configuration_aws_proto_rawDesc = []byte{
	0x0a, 0x24, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2f, 0x61, 0x77, 0x73, 0x2f, 0x63,
	0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f, 0x61, 0x77, 0x73,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x25, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2e, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2e, 0x61, 0x77, 0x73, 0x2e,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x1a, 0x1f, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xf0,
	0x01, 0x0a, 0x0d, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x12, 0x18, 0x0a, 0x07, 0x70, 0x72, 0x6f, 0x66, 0x69, 0x6c, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x07, 0x70, 0x72, 0x6f, 0x66, 0x69, 0x6c, 0x65, 0x12, 0x39, 0x0a, 0x19, 0x75, 0x73,
	0x65, 0x5f, 0x69, 0x6e, 0x6a, 0x65, 0x63, 0x74, 0x65, 0x64, 0x5f, 0x77, 0x65, 0x62, 0x5f, 0x69,
	0x64, 0x65, 0x6e, 0x74, 0x69, 0x74, 0x79, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x16, 0x75,
	0x73, 0x65, 0x49, 0x6e, 0x6a, 0x65, 0x63, 0x74, 0x65, 0x64, 0x57, 0x65, 0x62, 0x49, 0x64, 0x65,
	0x6e, 0x74, 0x69, 0x74, 0x79, 0x12, 0x26, 0x0a, 0x0f, 0x61, 0x73, 0x73, 0x75, 0x6d, 0x65, 0x5f,
	0x72, 0x6f, 0x6c, 0x65, 0x5f, 0x61, 0x72, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d,
	0x61, 0x73, 0x73, 0x75, 0x6d, 0x65, 0x52, 0x6f, 0x6c, 0x65, 0x41, 0x72, 0x6e, 0x12, 0x16, 0x0a,
	0x06, 0x72, 0x65, 0x67, 0x69, 0x6f, 0x6e, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x72,
	0x65, 0x67, 0x69, 0x6f, 0x6e, 0x12, 0x4a, 0x0a, 0x06, 0x73, 0x74, 0x61, 0x74, 0x69, 0x63, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x32, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2e, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2e, 0x61, 0x77, 0x73, 0x2e,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x43, 0x72,
	0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c, 0x73, 0x52, 0x06, 0x73, 0x74, 0x61, 0x74, 0x69,
	0x63, 0x22, 0xbe, 0x01, 0x0a, 0x0b, 0x43, 0x72, 0x65, 0x64, 0x65, 0x6e, 0x74, 0x69, 0x61, 0x6c,
	0x73, 0x12, 0x22, 0x0a, 0x0d, 0x61, 0x63, 0x63, 0x65, 0x73, 0x73, 0x5f, 0x6b, 0x65, 0x79, 0x5f,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x61, 0x63, 0x63, 0x65, 0x73, 0x73,
	0x4b, 0x65, 0x79, 0x49, 0x64, 0x12, 0x3a, 0x0a, 0x0a, 0x65, 0x78, 0x70, 0x69, 0x72, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54, 0x69, 0x6d, 0x65,
	0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x0a, 0x65, 0x78, 0x70, 0x69, 0x72, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x12, 0x2a, 0x0a, 0x11, 0x73, 0x65, 0x63, 0x72, 0x65, 0x74, 0x5f, 0x61, 0x63, 0x63, 0x65,
	0x73, 0x73, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x73, 0x65,
	0x63, 0x72, 0x65, 0x74, 0x41, 0x63, 0x63, 0x65, 0x73, 0x73, 0x4b, 0x65, 0x79, 0x12, 0x23, 0x0a,
	0x0d, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x5f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x04,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x54, 0x6f, 0x6b,
	0x65, 0x6e, 0x42, 0x39, 0x5a, 0x37, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c,
	0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2f, 0x75, 0x6e, 0x69, 0x76, 0x65, 0x72, 0x73, 0x65, 0x2f, 0x61, 0x77, 0x73, 0x2f,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_universe_aws_configuration_aws_proto_rawDescOnce sync.Once
	file_universe_aws_configuration_aws_proto_rawDescData = file_universe_aws_configuration_aws_proto_rawDesc
)

func file_universe_aws_configuration_aws_proto_rawDescGZIP() []byte {
	file_universe_aws_configuration_aws_proto_rawDescOnce.Do(func() {
		file_universe_aws_configuration_aws_proto_rawDescData = protoimpl.X.CompressGZIP(file_universe_aws_configuration_aws_proto_rawDescData)
	})
	return file_universe_aws_configuration_aws_proto_rawDescData
}

var file_universe_aws_configuration_aws_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_universe_aws_configuration_aws_proto_goTypes = []interface{}{
	(*Configuration)(nil),         // 0: foundation.universe.aws.configuration.Configuration
	(*Credentials)(nil),           // 1: foundation.universe.aws.configuration.Credentials
	(*timestamppb.Timestamp)(nil), // 2: google.protobuf.Timestamp
}
var file_universe_aws_configuration_aws_proto_depIdxs = []int32{
	1, // 0: foundation.universe.aws.configuration.Configuration.static:type_name -> foundation.universe.aws.configuration.Credentials
	2, // 1: foundation.universe.aws.configuration.Credentials.expiration:type_name -> google.protobuf.Timestamp
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_universe_aws_configuration_aws_proto_init() }
func file_universe_aws_configuration_aws_proto_init() {
	if File_universe_aws_configuration_aws_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_universe_aws_configuration_aws_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
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
		file_universe_aws_configuration_aws_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Credentials); i {
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
			RawDescriptor: file_universe_aws_configuration_aws_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_universe_aws_configuration_aws_proto_goTypes,
		DependencyIndexes: file_universe_aws_configuration_aws_proto_depIdxs,
		MessageInfos:      file_universe_aws_configuration_aws_proto_msgTypes,
	}.Build()
	File_universe_aws_configuration_aws_proto = out.File
	file_universe_aws_configuration_aws_proto_rawDesc = nil
	file_universe_aws_configuration_aws_proto_goTypes = nil
	file_universe_aws_configuration_aws_proto_depIdxs = nil
}