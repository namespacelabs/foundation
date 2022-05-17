// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1
// 	protoc        (unknown)
// source: devworkflow/types.proto

package devworkflow

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	schema "namespacelabs.dev/foundation/schema"
	protocol "namespacelabs.dev/foundation/workspace/tasks/protocol"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type Stack struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Revision      uint64                `protobuf:"varint,9,opt,name=revision,proto3" json:"revision,omitempty"`
	AbsRoot       string                `protobuf:"bytes,1,opt,name=abs_root,json=absRoot,proto3" json:"abs_root,omitempty"`
	Workspace     *schema.Workspace     `protobuf:"bytes,2,opt,name=workspace,proto3" json:"workspace,omitempty"`
	Env           *schema.Environment   `protobuf:"bytes,3,opt,name=env,proto3" json:"env,omitempty"`
	AvailableEnv  []*schema.Environment `protobuf:"bytes,8,rep,name=available_env,json=availableEnv,proto3" json:"available_env,omitempty"`
	Stack         *schema.Stack         `protobuf:"bytes,4,opt,name=stack,proto3" json:"stack,omitempty"`
	Current       *schema.Stack_Entry   `protobuf:"bytes,5,opt,name=current,proto3" json:"current,omitempty"`
	State         []*StackEntryState    `protobuf:"bytes,6,rep,name=state,proto3" json:"state,omitempty"`
	ForwardedPort []*ForwardedPort      `protobuf:"bytes,7,rep,name=forwarded_port,json=forwardedPort,proto3" json:"forwarded_port,omitempty"`
}

func (x *Stack) Reset() {
	*x = Stack{}
	if protoimpl.UnsafeEnabled {
		mi := &file_devworkflow_types_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Stack) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Stack) ProtoMessage() {}

func (x *Stack) ProtoReflect() protoreflect.Message {
	mi := &file_devworkflow_types_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Stack.ProtoReflect.Descriptor instead.
func (*Stack) Descriptor() ([]byte, []int) {
	return file_devworkflow_types_proto_rawDescGZIP(), []int{0}
}

func (x *Stack) GetRevision() uint64 {
	if x != nil {
		return x.Revision
	}
	return 0
}

func (x *Stack) GetAbsRoot() string {
	if x != nil {
		return x.AbsRoot
	}
	return ""
}

func (x *Stack) GetWorkspace() *schema.Workspace {
	if x != nil {
		return x.Workspace
	}
	return nil
}

func (x *Stack) GetEnv() *schema.Environment {
	if x != nil {
		return x.Env
	}
	return nil
}

func (x *Stack) GetAvailableEnv() []*schema.Environment {
	if x != nil {
		return x.AvailableEnv
	}
	return nil
}

func (x *Stack) GetStack() *schema.Stack {
	if x != nil {
		return x.Stack
	}
	return nil
}

func (x *Stack) GetCurrent() *schema.Stack_Entry {
	if x != nil {
		return x.Current
	}
	return nil
}

func (x *Stack) GetState() []*StackEntryState {
	if x != nil {
		return x.State
	}
	return nil
}

func (x *Stack) GetForwardedPort() []*ForwardedPort {
	if x != nil {
		return x.ForwardedPort
	}
	return nil
}

type ForwardedPort struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Endpoint      *schema.Endpoint `protobuf:"bytes,1,opt,name=endpoint,proto3" json:"endpoint,omitempty"`
	LocalPort     int32            `protobuf:"varint,2,opt,name=local_port,json=localPort,proto3" json:"local_port,omitempty"`
	ContainerPort int32            `protobuf:"varint,3,opt,name=container_port,json=containerPort,proto3" json:"container_port,omitempty"`
	Error         string           `protobuf:"bytes,4,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *ForwardedPort) Reset() {
	*x = ForwardedPort{}
	if protoimpl.UnsafeEnabled {
		mi := &file_devworkflow_types_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ForwardedPort) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ForwardedPort) ProtoMessage() {}

func (x *ForwardedPort) ProtoReflect() protoreflect.Message {
	mi := &file_devworkflow_types_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ForwardedPort.ProtoReflect.Descriptor instead.
func (*ForwardedPort) Descriptor() ([]byte, []int) {
	return file_devworkflow_types_proto_rawDescGZIP(), []int{1}
}

func (x *ForwardedPort) GetEndpoint() *schema.Endpoint {
	if x != nil {
		return x.Endpoint
	}
	return nil
}

func (x *ForwardedPort) GetLocalPort() int32 {
	if x != nil {
		return x.LocalPort
	}
	return 0
}

func (x *ForwardedPort) GetContainerPort() int32 {
	if x != nil {
		return x.ContainerPort
	}
	return 0
}

func (x *ForwardedPort) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

type Update struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	StackUpdate *Stack           `protobuf:"bytes,1,opt,name=stack_update,json=stackUpdate,proto3" json:"stack_update,omitempty"`
	TaskUpdate  []*protocol.Task `protobuf:"bytes,2,rep,name=task_update,json=taskUpdate,proto3" json:"task_update,omitempty"`
}

func (x *Update) Reset() {
	*x = Update{}
	if protoimpl.UnsafeEnabled {
		mi := &file_devworkflow_types_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Update) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Update) ProtoMessage() {}

func (x *Update) ProtoReflect() protoreflect.Message {
	mi := &file_devworkflow_types_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Update.ProtoReflect.Descriptor instead.
func (*Update) Descriptor() ([]byte, []int) {
	return file_devworkflow_types_proto_rawDescGZIP(), []int{2}
}

func (x *Update) GetStackUpdate() *Stack {
	if x != nil {
		return x.StackUpdate
	}
	return nil
}

func (x *Update) GetTaskUpdate() []*protocol.Task {
	if x != nil {
		return x.TaskUpdate
	}
	return nil
}

type StackEntryState struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PackageName string `protobuf:"bytes,1,opt,name=package_name,json=packageName,proto3" json:"package_name,omitempty"`
	LastError   string `protobuf:"bytes,2,opt,name=last_error,json=lastError,proto3" json:"last_error,omitempty"`
}

func (x *StackEntryState) Reset() {
	*x = StackEntryState{}
	if protoimpl.UnsafeEnabled {
		mi := &file_devworkflow_types_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StackEntryState) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StackEntryState) ProtoMessage() {}

func (x *StackEntryState) ProtoReflect() protoreflect.Message {
	mi := &file_devworkflow_types_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StackEntryState.ProtoReflect.Descriptor instead.
func (*StackEntryState) Descriptor() ([]byte, []int) {
	return file_devworkflow_types_proto_rawDescGZIP(), []int{3}
}

func (x *StackEntryState) GetPackageName() string {
	if x != nil {
		return x.PackageName
	}
	return ""
}

func (x *StackEntryState) GetLastError() string {
	if x != nil {
		return x.LastError
	}
	return ""
}

type DevWorkflowRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Type:
	//	*DevWorkflowRequest_SetWorkspace_
	//	*DevWorkflowRequest_ReloadWorkspace
	Type isDevWorkflowRequest_Type `protobuf_oneof:"type"`
}

func (x *DevWorkflowRequest) Reset() {
	*x = DevWorkflowRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_devworkflow_types_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DevWorkflowRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DevWorkflowRequest) ProtoMessage() {}

func (x *DevWorkflowRequest) ProtoReflect() protoreflect.Message {
	mi := &file_devworkflow_types_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DevWorkflowRequest.ProtoReflect.Descriptor instead.
func (*DevWorkflowRequest) Descriptor() ([]byte, []int) {
	return file_devworkflow_types_proto_rawDescGZIP(), []int{4}
}

func (m *DevWorkflowRequest) GetType() isDevWorkflowRequest_Type {
	if m != nil {
		return m.Type
	}
	return nil
}

func (x *DevWorkflowRequest) GetSetWorkspace() *DevWorkflowRequest_SetWorkspace {
	if x, ok := x.GetType().(*DevWorkflowRequest_SetWorkspace_); ok {
		return x.SetWorkspace
	}
	return nil
}

func (x *DevWorkflowRequest) GetReloadWorkspace() bool {
	if x, ok := x.GetType().(*DevWorkflowRequest_ReloadWorkspace); ok {
		return x.ReloadWorkspace
	}
	return false
}

type isDevWorkflowRequest_Type interface {
	isDevWorkflowRequest_Type()
}

type DevWorkflowRequest_SetWorkspace_ struct {
	SetWorkspace *DevWorkflowRequest_SetWorkspace `protobuf:"bytes,1,opt,name=set_workspace,json=setWorkspace,proto3,oneof"`
}

type DevWorkflowRequest_ReloadWorkspace struct {
	ReloadWorkspace bool `protobuf:"varint,2,opt,name=reload_workspace,json=reloadWorkspace,proto3,oneof"`
}

func (*DevWorkflowRequest_SetWorkspace_) isDevWorkflowRequest_Type() {}

func (*DevWorkflowRequest_ReloadWorkspace) isDevWorkflowRequest_Type() {}

type TerminalInput struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Stdin  []byte                      `protobuf:"bytes,1,opt,name=stdin,proto3" json:"stdin,omitempty"`
	Resize *TerminalInput_WindowResize `protobuf:"bytes,2,opt,name=resize,proto3" json:"resize,omitempty"`
}

func (x *TerminalInput) Reset() {
	*x = TerminalInput{}
	if protoimpl.UnsafeEnabled {
		mi := &file_devworkflow_types_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TerminalInput) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TerminalInput) ProtoMessage() {}

func (x *TerminalInput) ProtoReflect() protoreflect.Message {
	mi := &file_devworkflow_types_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TerminalInput.ProtoReflect.Descriptor instead.
func (*TerminalInput) Descriptor() ([]byte, []int) {
	return file_devworkflow_types_proto_rawDescGZIP(), []int{5}
}

func (x *TerminalInput) GetStdin() []byte {
	if x != nil {
		return x.Stdin
	}
	return nil
}

func (x *TerminalInput) GetResize() *TerminalInput_WindowResize {
	if x != nil {
		return x.Resize
	}
	return nil
}

type DevWorkflowRequest_SetWorkspace struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	AbsRoot     string `protobuf:"bytes,1,opt,name=abs_root,json=absRoot,proto3" json:"abs_root,omitempty"`
	PackageName string `protobuf:"bytes,2,opt,name=package_name,json=packageName,proto3" json:"package_name,omitempty"`
	EnvName     string `protobuf:"bytes,3,opt,name=env_name,json=envName,proto3" json:"env_name,omitempty"`
	Ephemeral   bool   `protobuf:"varint,5,opt,name=ephemeral,proto3" json:"ephemeral,omitempty"`
	// XXX this needs more appropriate modeling.
	AdditionalServers []string `protobuf:"bytes,4,rep,name=additional_servers,json=additionalServers,proto3" json:"additional_servers,omitempty"`
}

func (x *DevWorkflowRequest_SetWorkspace) Reset() {
	*x = DevWorkflowRequest_SetWorkspace{}
	if protoimpl.UnsafeEnabled {
		mi := &file_devworkflow_types_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DevWorkflowRequest_SetWorkspace) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DevWorkflowRequest_SetWorkspace) ProtoMessage() {}

func (x *DevWorkflowRequest_SetWorkspace) ProtoReflect() protoreflect.Message {
	mi := &file_devworkflow_types_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DevWorkflowRequest_SetWorkspace.ProtoReflect.Descriptor instead.
func (*DevWorkflowRequest_SetWorkspace) Descriptor() ([]byte, []int) {
	return file_devworkflow_types_proto_rawDescGZIP(), []int{4, 0}
}

func (x *DevWorkflowRequest_SetWorkspace) GetAbsRoot() string {
	if x != nil {
		return x.AbsRoot
	}
	return ""
}

func (x *DevWorkflowRequest_SetWorkspace) GetPackageName() string {
	if x != nil {
		return x.PackageName
	}
	return ""
}

func (x *DevWorkflowRequest_SetWorkspace) GetEnvName() string {
	if x != nil {
		return x.EnvName
	}
	return ""
}

func (x *DevWorkflowRequest_SetWorkspace) GetEphemeral() bool {
	if x != nil {
		return x.Ephemeral
	}
	return false
}

func (x *DevWorkflowRequest_SetWorkspace) GetAdditionalServers() []string {
	if x != nil {
		return x.AdditionalServers
	}
	return nil
}

type TerminalInput_WindowResize struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Width  uint32 `protobuf:"varint,1,opt,name=width,proto3" json:"width,omitempty"`
	Height uint32 `protobuf:"varint,2,opt,name=height,proto3" json:"height,omitempty"`
}

func (x *TerminalInput_WindowResize) Reset() {
	*x = TerminalInput_WindowResize{}
	if protoimpl.UnsafeEnabled {
		mi := &file_devworkflow_types_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TerminalInput_WindowResize) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TerminalInput_WindowResize) ProtoMessage() {}

func (x *TerminalInput_WindowResize) ProtoReflect() protoreflect.Message {
	mi := &file_devworkflow_types_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TerminalInput_WindowResize.ProtoReflect.Descriptor instead.
func (*TerminalInput_WindowResize) Descriptor() ([]byte, []int) {
	return file_devworkflow_types_proto_rawDescGZIP(), []int{5, 0}
}

func (x *TerminalInput_WindowResize) GetWidth() uint32 {
	if x != nil {
		return x.Width
	}
	return 0
}

func (x *TerminalInput_WindowResize) GetHeight() uint32 {
	if x != nil {
		return x.Height
	}
	return 0
}

var File_devworkflow_types_proto protoreflect.FileDescriptor

var file_devworkflow_types_proto_rawDesc = []byte{
	0x0a, 0x17, 0x64, 0x65, 0x76, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x2f, 0x74, 0x79,
	0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x16, 0x66, 0x6f, 0x75, 0x6e, 0x64,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x64, 0x65, 0x76, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f,
	0x77, 0x1a, 0x13, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x17, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x6e,
	0x65, 0x74, 0x77, 0x6f, 0x72, 0x6b, 0x69, 0x6e, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a,
	0x16, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2f, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x24, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61,
	0x63, 0x65, 0x2f, 0x74, 0x61, 0x73, 0x6b, 0x73, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f,
	0x6c, 0x2f, 0x74, 0x61, 0x73, 0x6b, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xe8, 0x03,
	0x0a, 0x05, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x12, 0x1a, 0x0a, 0x08, 0x72, 0x65, 0x76, 0x69, 0x73,
	0x69, 0x6f, 0x6e, 0x18, 0x09, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x72, 0x65, 0x76, 0x69, 0x73,
	0x69, 0x6f, 0x6e, 0x12, 0x19, 0x0a, 0x08, 0x61, 0x62, 0x73, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x62, 0x73, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x3a,
	0x0a, 0x09, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x1c, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73,
	0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x57, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x52,
	0x09, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x30, 0x0a, 0x03, 0x65, 0x6e,
	0x76, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x45, 0x6e, 0x76, 0x69,
	0x72, 0x6f, 0x6e, 0x6d, 0x65, 0x6e, 0x74, 0x52, 0x03, 0x65, 0x6e, 0x76, 0x12, 0x43, 0x0a, 0x0d,
	0x61, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x5f, 0x65, 0x6e, 0x76, 0x18, 0x08, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x45, 0x6e, 0x76, 0x69, 0x72, 0x6f, 0x6e, 0x6d,
	0x65, 0x6e, 0x74, 0x52, 0x0c, 0x61, 0x76, 0x61, 0x69, 0x6c, 0x61, 0x62, 0x6c, 0x65, 0x45, 0x6e,
	0x76, 0x12, 0x2e, 0x0a, 0x05, 0x73, 0x74, 0x61, 0x63, 0x6b, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x18, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63,
	0x68, 0x65, 0x6d, 0x61, 0x2e, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x52, 0x05, 0x73, 0x74, 0x61, 0x63,
	0x6b, 0x12, 0x38, 0x0a, 0x07, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e,
	0x73, 0x63, 0x68, 0x65, 0x6d, 0x61, 0x2e, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x2e, 0x45, 0x6e, 0x74,
	0x72, 0x79, 0x52, 0x07, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x12, 0x3d, 0x0a, 0x05, 0x73,
	0x74, 0x61, 0x74, 0x65, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x27, 0x2e, 0x66, 0x6f, 0x75,
	0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x64, 0x65, 0x76, 0x77, 0x6f, 0x72, 0x6b, 0x66,
	0x6c, 0x6f, 0x77, 0x2e, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x53, 0x74,
	0x61, 0x74, 0x65, 0x52, 0x05, 0x73, 0x74, 0x61, 0x74, 0x65, 0x12, 0x4c, 0x0a, 0x0e, 0x66, 0x6f,
	0x72, 0x77, 0x61, 0x72, 0x64, 0x65, 0x64, 0x5f, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x07, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x25, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e,
	0x64, 0x65, 0x76, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x2e, 0x46, 0x6f, 0x72, 0x77,
	0x61, 0x72, 0x64, 0x65, 0x64, 0x50, 0x6f, 0x72, 0x74, 0x52, 0x0d, 0x66, 0x6f, 0x72, 0x77, 0x61,
	0x72, 0x64, 0x65, 0x64, 0x50, 0x6f, 0x72, 0x74, 0x22, 0xa4, 0x01, 0x0a, 0x0d, 0x46, 0x6f, 0x72,
	0x77, 0x61, 0x72, 0x64, 0x65, 0x64, 0x50, 0x6f, 0x72, 0x74, 0x12, 0x37, 0x0a, 0x08, 0x65, 0x6e,
	0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x66,
	0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x73, 0x63, 0x68, 0x65, 0x6d, 0x61,
	0x2e, 0x45, 0x6e, 0x64, 0x70, 0x6f, 0x69, 0x6e, 0x74, 0x52, 0x08, 0x65, 0x6e, 0x64, 0x70, 0x6f,
	0x69, 0x6e, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x5f, 0x70, 0x6f, 0x72,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x6c, 0x6f, 0x63, 0x61, 0x6c, 0x50, 0x6f,
	0x72, 0x74, 0x12, 0x25, 0x0a, 0x0e, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x5f,
	0x70, 0x6f, 0x72, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0d, 0x63, 0x6f, 0x6e, 0x74,
	0x61, 0x69, 0x6e, 0x65, 0x72, 0x50, 0x6f, 0x72, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x65, 0x72, 0x72,
	0x6f, 0x72, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x22,
	0x96, 0x01, 0x0a, 0x06, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x40, 0x0a, 0x0c, 0x73, 0x74,
	0x61, 0x63, 0x6b, 0x5f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1d, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x64, 0x65,
	0x76, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x2e, 0x53, 0x74, 0x61, 0x63, 0x6b, 0x52,
	0x0b, 0x73, 0x74, 0x61, 0x63, 0x6b, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x12, 0x4a, 0x0a, 0x0b,
	0x74, 0x61, 0x73, 0x6b, 0x5f, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x29, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x77,
	0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x2e, 0x74, 0x61, 0x73, 0x6b, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x63, 0x6f, 0x6c, 0x2e, 0x54, 0x61, 0x73, 0x6b, 0x52, 0x0a, 0x74, 0x61,
	0x73, 0x6b, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x22, 0x53, 0x0a, 0x0f, 0x53, 0x74, 0x61, 0x63,
	0x6b, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x70,
	0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0b, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1d,
	0x0a, 0x0a, 0x6c, 0x61, 0x73, 0x74, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x09, 0x6c, 0x61, 0x73, 0x74, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x22, 0xe0, 0x02,
	0x0a, 0x12, 0x44, 0x65, 0x76, 0x57, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x5e, 0x0a, 0x0d, 0x73, 0x65, 0x74, 0x5f, 0x77, 0x6f, 0x72, 0x6b,
	0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x37, 0x2e, 0x66, 0x6f,
	0x75, 0x6e, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x64, 0x65, 0x76, 0x77, 0x6f, 0x72, 0x6b,
	0x66, 0x6c, 0x6f, 0x77, 0x2e, 0x44, 0x65, 0x76, 0x57, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x2e, 0x53, 0x65, 0x74, 0x57, 0x6f, 0x72, 0x6b, 0x73,
	0x70, 0x61, 0x63, 0x65, 0x48, 0x00, 0x52, 0x0c, 0x73, 0x65, 0x74, 0x57, 0x6f, 0x72, 0x6b, 0x73,
	0x70, 0x61, 0x63, 0x65, 0x12, 0x2b, 0x0a, 0x10, 0x72, 0x65, 0x6c, 0x6f, 0x61, 0x64, 0x5f, 0x77,
	0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x48, 0x00,
	0x52, 0x0f, 0x72, 0x65, 0x6c, 0x6f, 0x61, 0x64, 0x57, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x1a, 0xb4, 0x01, 0x0a, 0x0c, 0x53, 0x65, 0x74, 0x57, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61,
	0x63, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x61, 0x62, 0x73, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x61, 0x62, 0x73, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x21, 0x0a,
	0x0c, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0b, 0x70, 0x61, 0x63, 0x6b, 0x61, 0x67, 0x65, 0x4e, 0x61, 0x6d, 0x65,
	0x12, 0x19, 0x0a, 0x08, 0x65, 0x6e, 0x76, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x65, 0x6e, 0x76, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1c, 0x0a, 0x09, 0x65,
	0x70, 0x68, 0x65, 0x6d, 0x65, 0x72, 0x61, 0x6c, 0x18, 0x05, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09,
	0x65, 0x70, 0x68, 0x65, 0x6d, 0x65, 0x72, 0x61, 0x6c, 0x12, 0x2d, 0x0a, 0x12, 0x61, 0x64, 0x64,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x61, 0x6c, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x73, 0x18,
	0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x11, 0x61, 0x64, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x61,
	0x6c, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72, 0x73, 0x42, 0x06, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65,
	0x22, 0xaf, 0x01, 0x0a, 0x0d, 0x54, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x61, 0x6c, 0x49, 0x6e, 0x70,
	0x75, 0x74, 0x12, 0x14, 0x0a, 0x05, 0x73, 0x74, 0x64, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0c, 0x52, 0x05, 0x73, 0x74, 0x64, 0x69, 0x6e, 0x12, 0x4a, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x69,
	0x7a, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x32, 0x2e, 0x66, 0x6f, 0x75, 0x6e, 0x64,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x64, 0x65, 0x76, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f,
	0x77, 0x2e, 0x54, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x61, 0x6c, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x2e,
	0x57, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x52, 0x65, 0x73, 0x69, 0x7a, 0x65, 0x52, 0x06, 0x72, 0x65,
	0x73, 0x69, 0x7a, 0x65, 0x1a, 0x3c, 0x0a, 0x0c, 0x57, 0x69, 0x6e, 0x64, 0x6f, 0x77, 0x52, 0x65,
	0x73, 0x69, 0x7a, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x77, 0x69, 0x64, 0x74, 0x68, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0d, 0x52, 0x05, 0x77, 0x69, 0x64, 0x74, 0x68, 0x12, 0x16, 0x0a, 0x06, 0x68, 0x65,
	0x69, 0x67, 0x68, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0d, 0x52, 0x06, 0x68, 0x65, 0x69, 0x67,
	0x68, 0x74, 0x42, 0x2a, 0x5a, 0x28, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x6c,
	0x61, 0x62, 0x73, 0x2e, 0x64, 0x65, 0x76, 0x2f, 0x66, 0x6f, 0x75, 0x6e, 0x64, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x2f, 0x64, 0x65, 0x76, 0x77, 0x6f, 0x72, 0x6b, 0x66, 0x6c, 0x6f, 0x77, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_devworkflow_types_proto_rawDescOnce sync.Once
	file_devworkflow_types_proto_rawDescData = file_devworkflow_types_proto_rawDesc
)

func file_devworkflow_types_proto_rawDescGZIP() []byte {
	file_devworkflow_types_proto_rawDescOnce.Do(func() {
		file_devworkflow_types_proto_rawDescData = protoimpl.X.CompressGZIP(file_devworkflow_types_proto_rawDescData)
	})
	return file_devworkflow_types_proto_rawDescData
}

var file_devworkflow_types_proto_msgTypes = make([]protoimpl.MessageInfo, 8)
var file_devworkflow_types_proto_goTypes = []interface{}{
	(*Stack)(nil),                           // 0: foundation.devworkflow.Stack
	(*ForwardedPort)(nil),                   // 1: foundation.devworkflow.ForwardedPort
	(*Update)(nil),                          // 2: foundation.devworkflow.Update
	(*StackEntryState)(nil),                 // 3: foundation.devworkflow.StackEntryState
	(*DevWorkflowRequest)(nil),              // 4: foundation.devworkflow.DevWorkflowRequest
	(*TerminalInput)(nil),                   // 5: foundation.devworkflow.TerminalInput
	(*DevWorkflowRequest_SetWorkspace)(nil), // 6: foundation.devworkflow.DevWorkflowRequest.SetWorkspace
	(*TerminalInput_WindowResize)(nil),      // 7: foundation.devworkflow.TerminalInput.WindowResize
	(*schema.Workspace)(nil),                // 8: foundation.schema.Workspace
	(*schema.Environment)(nil),              // 9: foundation.schema.Environment
	(*schema.Stack)(nil),                    // 10: foundation.schema.Stack
	(*schema.Stack_Entry)(nil),              // 11: foundation.schema.Stack.Entry
	(*schema.Endpoint)(nil),                 // 12: foundation.schema.Endpoint
	(*protocol.Task)(nil),                   // 13: foundation.workspace.tasks.protocol.Task
}
var file_devworkflow_types_proto_depIdxs = []int32{
	8,  // 0: foundation.devworkflow.Stack.workspace:type_name -> foundation.schema.Workspace
	9,  // 1: foundation.devworkflow.Stack.env:type_name -> foundation.schema.Environment
	9,  // 2: foundation.devworkflow.Stack.available_env:type_name -> foundation.schema.Environment
	10, // 3: foundation.devworkflow.Stack.stack:type_name -> foundation.schema.Stack
	11, // 4: foundation.devworkflow.Stack.current:type_name -> foundation.schema.Stack.Entry
	3,  // 5: foundation.devworkflow.Stack.state:type_name -> foundation.devworkflow.StackEntryState
	1,  // 6: foundation.devworkflow.Stack.forwarded_port:type_name -> foundation.devworkflow.ForwardedPort
	12, // 7: foundation.devworkflow.ForwardedPort.endpoint:type_name -> foundation.schema.Endpoint
	0,  // 8: foundation.devworkflow.Update.stack_update:type_name -> foundation.devworkflow.Stack
	13, // 9: foundation.devworkflow.Update.task_update:type_name -> foundation.workspace.tasks.protocol.Task
	6,  // 10: foundation.devworkflow.DevWorkflowRequest.set_workspace:type_name -> foundation.devworkflow.DevWorkflowRequest.SetWorkspace
	7,  // 11: foundation.devworkflow.TerminalInput.resize:type_name -> foundation.devworkflow.TerminalInput.WindowResize
	12, // [12:12] is the sub-list for method output_type
	12, // [12:12] is the sub-list for method input_type
	12, // [12:12] is the sub-list for extension type_name
	12, // [12:12] is the sub-list for extension extendee
	0,  // [0:12] is the sub-list for field type_name
}

func init() { file_devworkflow_types_proto_init() }
func file_devworkflow_types_proto_init() {
	if File_devworkflow_types_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_devworkflow_types_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Stack); i {
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
		file_devworkflow_types_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ForwardedPort); i {
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
		file_devworkflow_types_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Update); i {
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
		file_devworkflow_types_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StackEntryState); i {
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
		file_devworkflow_types_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DevWorkflowRequest); i {
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
		file_devworkflow_types_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TerminalInput); i {
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
		file_devworkflow_types_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DevWorkflowRequest_SetWorkspace); i {
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
		file_devworkflow_types_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TerminalInput_WindowResize); i {
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
	file_devworkflow_types_proto_msgTypes[4].OneofWrappers = []interface{}{
		(*DevWorkflowRequest_SetWorkspace_)(nil),
		(*DevWorkflowRequest_ReloadWorkspace)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_devworkflow_types_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   8,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_devworkflow_types_proto_goTypes,
		DependencyIndexes: file_devworkflow_types_proto_depIdxs,
		MessageInfos:      file_devworkflow_types_proto_msgTypes,
	}.Build()
	File_devworkflow_types_proto = out.File
	file_devworkflow_types_proto_rawDesc = nil
	file_devworkflow_types_proto_goTypes = nil
	file_devworkflow_types_proto_depIdxs = nil
}
