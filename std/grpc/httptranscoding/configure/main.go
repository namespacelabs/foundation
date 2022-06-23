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
	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime"
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

	computedNaming := &schema.ComputedNaming{}
	if ok, err := req.CheckUnpackInput(computedNaming); err != nil {
		return err
	} else if !ok {
		return fnerrors.InternalError("ComputedNaming is required")
	}

	type service struct {
		ProtoService string
		Server       *schema.Stack_Entry
		Endpoint     *schema.Endpoint
		Transcoding  *schema.GrpcHttpTranscoding
	}

	var endpoints []service
	for _, endpoint := range req.Stack.Endpoint {
		var protoService string
		for _, md := range endpoint.ServiceMetadata {
			if md.Protocol == schema.GrpcProtocol {
				protoService = md.Kind
				break
			}
		}

		if endpoint.GetServerOwnerPackage() != req.Focus.GetPackageName() {
			continue
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

			endpoints = append(endpoints, service{
				ProtoService: protoService,
				Server:       req.Focus,
				Endpoint:     endpoint,
				Transcoding:  t,
			})

			break
		}
	}

	// This block embeds runtime logic into the transcoding handler; this means
	// that version will be at odds with the fn scheduler. Ideally we'd receive
	// these set of domains in.
	ingressName := fmt.Sprintf("grpc-gateway-%s", req.Focus.Server.Id)
	domains, err := runtime.CalculateDomains(req.Env, computedNaming, ingressName)
	if err != nil {
		return err
	}

	cors := &schema.HttpCors{Enabled: true, AllowedOrigin: []string{"*"}}
	packedCors, err := anypb.New(cors)
	if err != nil {
		return fnerrors.UserError(nil, "failed to pack CORS' configuration: %v", err)
	}

	for _, x := range endpoints {
		fds, err := proto.Marshal(x.Transcoding.FileDescriptorSet)
		if err != nil {
			return fnerrors.New("failed to marshal FileDescriptorSet: %w", err)
		}

		annotations, err := kubedef.MakeServiceAnnotations(x.Server.Server, x.Endpoint)
		if err != nil {
			return fnerrors.New("failed to calculate annotations: %w", err)
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
			// This instructs the runtime to wait until the CRD above has a
			// status.conditions of type Applied, which matches the observed
			// generation.
			CheckGenerationCondition: &kubedef.CheckGenerationCondition{
				Type: "Applied",
			},
		})

		// Only emit ingress entries for internet facing services.
		if x.Endpoint.Type != schema.Endpoint_INTERNET_FACING {
			continue
		}

		// XXX handle method level routing.
		for _, domain := range domains {
			fragment, err := anypb.New(&schema.IngressFragment{
				Name:     ingressName,
				Domain:   domain,
				Owner:    req.Focus.Server.PackageName,
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
