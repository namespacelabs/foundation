// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

const (
	transcoderServiceName = "grpc-http-transcoder"
	gatewayServer         = "namespacelabs.dev/foundation/std/networking/gateway/server"
)

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	configure.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	var transcoderEndpoint *schema.Endpoint
	for _, endpoint := range req.Stack.Endpoint {
		if endpoint.ServerOwner == gatewayServer && endpoint.ServiceName == transcoderServiceName {
			transcoderEndpoint = endpoint
			break
		}
	}

	if transcoderEndpoint == nil {
		return fnerrors.New("%s: missing endpoint", transcoderServiceName)
	}

	type endpoint struct {
		ProtoService string
		Server       *schema.Stack_Entry
		Endpoint     *schema.Endpoint
		Transcoding  *schema.GrpcHttpTranscoding
	}

	perServer := map[string][]endpoint{}

	for _, e := range req.Stack.Endpoint {
		var protoService string
		for _, md := range e.ServiceMetadata {
			if md.Protocol == schema.GrpcProtocol {
				protoService = md.Kind
				break
			}
		}

		sch := req.Stack.GetServer(e.GetServerOwnerPackage())
		if sch == nil {
			continue
		}

		if protoService == "" || e.Port == nil {
			continue
		}

		for _, md := range e.ServiceMetadata {
			t := &schema.GrpcHttpTranscoding{}
			if !md.Details.MessageIs(t) {
				continue
			}

			if err := md.Details.UnmarshalTo(t); err != nil {
				return fnerrors.New("failed to unmarshal GrpcHttpTranscoding: %w", err)
			}

			perServer[e.ServerOwner] = append(perServer[e.ServerOwner], endpoint{
				ProtoService: protoService,
				Server:       sch,
				Endpoint:     e,
				Transcoding:  t,
			})
		}
	}

	var sorted [][]endpoint
	for _, services := range perServer {
		if len(services) > 0 {
			sorted = append(sorted, services)
		}
	}

	slices.SortFunc(sorted, func(a, b []endpoint) bool {
		// Each slice of endpoints is guaranteed to be of the same server.
		return strings.Compare(a[0].Endpoint.ServerOwner, b[0].Endpoint.ServerOwner) < 0
	})

	for _, serverEndpoints := range sorted {
		for _, x := range serverEndpoints {
			fds, err := proto.Marshal(x.Transcoding.FileDescriptorSet)
			if err != nil {
				return fnerrors.New("failed to marshal FileDescriptorSet: %w", err)
			}

			annotations, err := kubedef.MakeServiceAnnotations(x.Server.Server, x.Endpoint)
			if err != nil {
				return fnerrors.New("failed to calculation annotations: %w", err)
			}

			out.Invocations = append(out.Invocations, kubedef.Apply{
				Description: fmt.Sprintf("HTTP/gRPC transcoder: %s", x.ProtoService),
				Resource: &httpGrpcTranscoder{
					TypeMeta: metav1.TypeMeta{
						Kind:       "HttpGrpcTranscoder",
						APIVersion: "k8s.namespacelabs.dev/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:        strings.ToLower(fmt.Sprintf("%s-%s", x.ProtoService, x.Server.Server.Id)),
						Namespace:   kubetool.FromRequest(req).Namespace,
						Labels:      kubedef.MakeLabels(req.Env, x.Server.Server),
						Annotations: annotations,
					},
					Spec: httpGrpcTranscoderSpec{
						FullyQualifiedProtoServiceName: x.ProtoService,
						ServiceAddress:                 x.Endpoint.AllocatedName,
						ServicePort:                    int(x.Endpoint.Port.ContainerPort),
						EncodedProtoDescriptor:         base64.StdEncoding.EncodeToString(fds),
					},
				},
				ResourceClass: &kubedef.ResourceClass{
					Resource: "httpgrpctranscoders",
					Group:    "k8s.namespacelabs.dev",
					Version:  "v1",
				},
			})

			// Only emit ingress entries for internet facing services.
			if x.Endpoint.Type != schema.Endpoint_INTERNET_FACING {
				continue
			}

			cors := &schema.HttpCors{Enabled: true, AllowedOrigin: []string{"*"}}
			packedCors, err := anypb.New(cors)
			if err != nil {
				return fnerrors.UserError(nil, "failed to pack CORS' configuration: %v", err)
			}

			srv := x.Server.Server
			name := fmt.Sprintf("grpc-gateway-%s", srv.Id)
			fragment, err := anypb.New(&schema.IngressFragmentPlan{
				AllocatedName: name,
				IngressFragment: &schema.IngressFragment{
					Name:     name,
					Owner:    srv.PackageName,
					Endpoint: x.Endpoint,
					// Point HTTP calls under /{serviceName}/ to Envoy.
					HttpPath: []*schema.IngressFragment_IngressHttpPath{{
						Path:    fmt.Sprintf("/%s/", x.ProtoService),
						Owner:   x.Endpoint.EndpointOwner,
						Service: transcoderEndpoint.AllocatedName,
						Port:    transcoderEndpoint.Port,
					}},
					Extension: []*anypb.Any{packedCors},
					Manager:   "namespacelabs.dev/foundation/std/grpc/httptranscoding",
				},
			})
			if err != nil {
				return err
			}

			out.Computed = append(out.Computed, &schema.ComputedConfiguration{
				Owner: "namespacelabs.dev/foundation/std/grpc/httptranscoding",
				Impl:  fragment,
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

	Spec httpGrpcTranscoderSpec `json:"spec"`
}

type httpGrpcTranscoderSpec struct {
	FullyQualifiedProtoServiceName string `json:"fullyQualifiedProtoServiceName"`
	ServiceAddress                 string `json:"serviceAddress"`
	ServicePort                    int    `json:"servicePort"`
	EncodedProtoDescriptor         string `json:"encodedProtoDescriptor"`
}
