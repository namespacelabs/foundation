// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gke

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/providers/nscloud/nsingress"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/shared"
	"namespacelabs.dev/foundation/schema"
)

type gclb struct {
	shared.MapPublicLoadBalancer
}

func (gclb) ComputeNaming(env *schema.Environment, naming *schema.Naming) (*schema.ComputedNaming, error) {
	return nsingress.ComputeNaming(env, naming)
}

func (gclb) Ensure(context.Context) ([]*schema.SerializedInvocation, error) {
	// XXX validate that cluster is gke.
	return nil, nil
}
func (gclb) Service() *kubedef.IngressSelector             { return nil }
func (gclb) Waiter(*rest.Config) kubedef.KubeIngressWaiter { return nil }

func (gclb) IngressAnnotations(hasTLS bool, backendProtocol kubedef.BackendProtocol, extensions []*anypb.Any) (map[string]string, error) {
	annotations := kubedef.BaseAnnotations()

	annotations["kubernetes.io/ingress.class"] = "gce"

	if backendProtocol != kubedef.BackendProtocol_HTTP {
		return nil, fnerrors.BadInputError("only support backend protocol %q, got %q", kubedef.BackendProtocol_HTTP, backendProtocol)
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

	return annotations, nil
}
