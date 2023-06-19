// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubetool"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/networking/ingress/nginx"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
)

const (
	transcoderServiceName = "grpc-http-transcoder"
	gatewayServer         = "namespacelabs.dev/foundation/std/networking/gateway/server"
)

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	provisioning.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
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
		GrpcProtocol string
	}

	var endpoints []service
	for _, endpoint := range req.Stack.Endpoint {
		var protoService, grpcProtocol string
		for _, md := range endpoint.ServiceMetadata {
			if md.Protocol == schema.ClearTextGrpcProtocol || md.Protocol == schema.GrpcProtocol {
				protoService = md.Kind
				grpcProtocol = md.Protocol
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
				GrpcProtocol: grpcProtocol,
			})

			break
		}
	}

	// This block embeds runtime logic into the transcoding handler; this means
	// that version will be at odds with the fn scheduler. Ideally we'd receive
	// these set of domains in.
	ingressName := fmt.Sprintf("grpc-gateway-%s", req.Focus.Server.Id)

	// If a suffix is being used, constrain the prefix size.
	// XXX work in progress.
	if computedNaming.DomainFragmentSuffix != "" {
		ingressName = "grpc-" + req.Focus.Server.Id[:4]
	}

	domains, err := runtime.CalculateDomains(req.Env, computedNaming, runtime.DomainsRequest{
		ServerID: req.Focus.Server.Id,
		Key:      ingressName,
		Alias:    "grpc",
		// XXX UserSpecified.
	})
	if err != nil {
		return err
	}

	proxyBodySize := &nginx.ProxyBodySize{
		// Remove the body size limit on nginx level. This is useful for large streaming RPCs as nginx limit
		// applies to the cumulative body size constructed out of all the JSON payloads.
		// Note Envoy still improses the (default) buffer limit of 1M on unary requests and individual payloads.
		Limit: "0",
	}
	packedProxyBodySize, err := anypb.New(proxyBodySize)
	if err != nil {
		return fnerrors.New("failed to pack ProxyBodySize configuration: %v", err)
	}

	cors := &schema.HttpCors{Enabled: true, AllowedOrigin: []string{"*"}, ExposeHeaders: []string{"*"}}
	packedCors, err := anypb.New(cors)
	if err != nil {
		return fnerrors.New("failed to pack CORS' configuration: %v", err)
	}

	kr, err := kubetool.FromRequest(req)
	if err != nil {
		return err
	}

	for _, x := range endpoints {
		fds, err := proto.Marshal(x.Transcoding.FileDescriptorSet)
		if err != nil {
			return fnerrors.New("failed to marshal FileDescriptorSet: %w", err)
		}

		annotations, err := kubedef.MakeServiceAnnotations(x.Endpoint)
		if err != nil {
			return fnerrors.New("failed to calculate annotations: %w", err)
		}

		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description:  fmt.Sprintf("HTTP/gRPC transcoder: %s", x.ProtoService),
			SetNamespace: kr.CanSetNamespace,
			Resource: &httpGrpcTranscoder{
				TypeMeta: metav1.TypeMeta{
					Kind:       "HttpGrpcTranscoder",
					APIVersion: "k8s.namespacelabs.dev/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        strings.ToLower(fmt.Sprintf("%s-%s", x.ProtoService, x.Server.Server.Id)),
					Namespace:   kr.Namespace,
					Labels:      kubedef.MakeLabels(req.Env, x.Server.Server),
					Annotations: annotations,
				},
				Spec: httpGrpcTranscoderSpec{
					FullyQualifiedProtoServiceName: x.ProtoService,
					ServiceAddress:                 x.Endpoint.AllocatedName,
					ServicePort:                    int(x.Endpoint.Port.ContainerPort),
					BackendTLS:                     x.GrpcProtocol == schema.GrpcProtocol,
					EncodedProtoDescriptor:         base64.StdEncoding.EncodeToString(fds),
				},
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
					Path:        fmt.Sprintf("/%s/", x.ProtoService),
					Owner:       x.Endpoint.EndpointOwner,
					Service:     transcoderEndpoint.AllocatedName,
					ServicePort: transcoderEndpoint.GetExportedPort(),
				}},
				Extension: []*anypb.Any{packedCors, packedProxyBodySize},
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

func (configuration) Delete(context.Context, provisioning.StackRequest, *provisioning.DeleteOutput) error {
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
	BackendTLS                     bool   `json:"backendTls"`
	EncodedProtoDescriptor         string `json:"encodedProtoDescriptor"`
}
