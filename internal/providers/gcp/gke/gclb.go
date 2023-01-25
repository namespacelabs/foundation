// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gke

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/schema"
)

type gclb struct{}

func (gclb) ComputeNaming(env *schema.Environment, naming *schema.Naming) (*schema.ComputedNaming, error) {
	if naming.GetWithOrg() != "" {
		return nil, fnerrors.InternalError("nscloud tls allocation not supported with gclb")
	}

	return &schema.ComputedNaming{Source: naming}, nil
}

func (gclb) Ensure(context.Context) ([]*schema.SerializedInvocation, error) {
	// XXX validate that cluster is gke.
	return nil, nil
}
func (gclb) Service() *kubedef.IngressSelector             { return nil }
func (gclb) Waiter(*rest.Config) kubedef.KubeIngressWaiter { return nil }

func (gclb) PrepareRoute(ctx context.Context, _ *schema.Environment, _ *schema.Stack_Entry, domain *schema.Domain, ns, name string) (*kubedef.IngressAllocatedRoute, error) {
	return nil, nil
}

func (gclb) Annotate(ns, name string, domains []*schema.Domain, hasTLS bool, backendProtocol kubedef.BackendProtocol, extensions []*anypb.Any) (*kubedef.IngressAnnotations, error) {
	ann := &kubedef.IngressAnnotations{
		Annotations: kubedef.BaseAnnotations(),
	}

	ann.Annotations["kubernetes.io/ingress.class"] = "gce"

	if backendProtocol != kubedef.BackendProtocol_HTTP {
		return nil, fnerrors.BadInputError("only support backend protocol %q, got %q", kubedef.BackendProtocol_HTTP, backendProtocol)
	}

	var domainList uniquestrings.List
	for _, domain := range domains {
		if domain.TlsFrontend {
			domainList.Add(domain.Fqdn)
		}
	}

	if domainList.Len() > 0 {
		domains := domainList.Strings()
		certname := fmt.Sprintf("%s-certs", name)
		cat := fmt.Sprintf("gclb:ManagedCertificate:%s", certname)

		ann.Resources = append(ann.Resources, kubedef.Apply{
			Description: fmt.Sprintf("Google Cloud ManagedCertificate: %s", strings.Join(domains, ", ")),
			// JSON is used because the Google-provided typed ManagedCertificate has a Status object which
			// is not default ommited. Which messes up our applies.
			Resource: map[string]any{
				"kind":       "ManagedCertificate",
				"apiVersion": "networking.gke.io/v1",
				"metadata": map[string]any{
					"name":      certname,
					"namespace": ns,
				},
				"spec": map[string]any{
					"domains": domains,
				},
			},
			SchedCategory: []string{cat},
		})

		ann.Annotations["networking.gke.io/managed-certificates"] = certname
		ann.SchedAfter = append(ann.SchedAfter, cat)
	}

	// if hasTLS {
	// 	Use FrontendConfig.redirectToHttps
	// }

	// XXX cors is skipped for now.
	var cors *schema.HttpCors

	for _, ext := range extensions {
		msg, err := ext.UnmarshalNew()
		if err != nil {
			return nil, fnerrors.InternalError("gclb: failed to unpack configuration: %v", err)
		}

		switch x := msg.(type) {
		case *schema.HttpCors:
			if !protos.CheckConsolidate(x, &cors) {
				return nil, fnerrors.InternalError("gclb: incompatible CORS configurations")
			}

		default:
			return nil, fnerrors.InternalError("gclb: don't know how to handle extension %q", ext.TypeUrl)
		}
	}

	return ann, nil
}
