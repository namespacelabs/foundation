// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type HttpGrpcTranscoder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	FullyQualifiedProtoServiceName string `json:"fullyQualifiedProtoServiceName,omitempty"`
	ServiceAddress                 string `json:"serviceAddress,omitempty"`
	ServicePort                    int    `json:"servicePort,omitempty"`
	EncodedProtoDescriptor         string `json:"encodedProtoDescriptor,omitempty"`
}

// DeepCopyInto, DeepCopy, and DeepCopyObject are generated typically with
// https://github.com/kubernetes/code-generator and are necessary to fulfil the API contract
// for custom resources.
func (in *HttpGrpcTranscoder) DeepCopyInto(out *HttpGrpcTranscoder) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	out.FullyQualifiedProtoServiceName = in.FullyQualifiedProtoServiceName
	out.ServiceAddress = in.ServiceAddress
	out.ServicePort = in.ServicePort
	out.EncodedProtoDescriptor = in.EncodedProtoDescriptor
}

func (in *HttpGrpcTranscoder) DeepCopy() *HttpGrpcTranscoder {
	if in == nil {
		return nil
	}
	out := new(HttpGrpcTranscoder)
	in.DeepCopyInto(out)
	return out
}

func (in *HttpGrpcTranscoder) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type HttpGrpcTranscoderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HttpGrpcTranscoder `json:"items"`
}

func (in *HttpGrpcTranscoderList) DeepCopyInto(out *HttpGrpcTranscoderList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]HttpGrpcTranscoder, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *HttpGrpcTranscoderList) DeepCopy() *HttpGrpcTranscoderList {
	if in == nil {
		return nil
	}
	out := new(HttpGrpcTranscoderList)
	in.DeepCopyInto(out)
	return out
}

func (in *HttpGrpcTranscoderList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
