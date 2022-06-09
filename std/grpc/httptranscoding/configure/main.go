// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	configure.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	for _, endpoint := range req.Stack.Endpoint {
		if endpoint.ServerOwner != req.Focus.Server.PackageName {
			continue
		}

		var protoService string
		for _, md := range endpoint.ServiceMetadata {
			if md.Protocol == schema.GrpcProtocol {
				protoService = md.Kind
				break
			}
		}

		if protoService == "" || endpoint.Port == nil {
			continue
		}

		for _, md := range endpoint.ServiceMetadata {
			t := &schema.GrpcHttpTranscoding{}
			if !md.Details.MessageIs(t) {
				continue
			}

			if err := md.Details.UnmarshalTo(t); err != nil {
				return fnerrors.New("failed to unmarshal GrpcHttpTranscoding: %w", err)
			}

			fds, err := proto.Marshal(t.FileDescriptorSet)
			if err != nil {
				return fnerrors.New("failed to marshal FileDescriptorSet: %w", err)
			}

			out.Invocations = append(out.Invocations, kubedef.Apply{
				Description: fmt.Sprintf("HTTP/gRPC transcoder: %s", protoService),
				Resource: &httpGrpcTranscoder{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HttpGrpcTranscoder",
						APIVersion: "k8s.namespacelabs.dev/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      strings.ToLower(fmt.Sprintf("%s-%s", protoService, req.Focus.Server.Id)),
						Namespace: kubetool.FromRequest(req).Namespace,
						Labels:    kubedef.MakeLabels(req.Env, req.Focus.Server),
					},
					FullyQualifiedProtoServiceName: protoService,
					ServiceAddress:                 endpoint.AllocatedName,
					ServicePort:                    int(endpoint.Port.ContainerPort),
					EncodedProtoDescriptor:         base64.StdEncoding.EncodeToString(fds),
				},
				ResourceClass: &kubedef.ResourceClass{
					Resource: "httpgrpctranscoders",
					Group:    "k8s.namespacelabs.dev",
					Version:  "v1",
				},
			})

		}
	}

	return nil
}

func (configuration) Delete(context.Context, configure.StackRequest, *configure.DeleteOutput) error {
	// XXX unimplemented
	return nil
}

type httpGrpcTranscoder struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	FullyQualifiedProtoServiceName string `json:"fullyQualifiedProtoServiceName"`
	ServiceAddress                 string `json:"serviceAddress"`
	ServicePort                    int    `json:"servicePort"`
	EncodedProtoDescriptor         string `json:"encodedProtoDescriptor"`
}
