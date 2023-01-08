// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubedef

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/client-go/rest"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/execution/defs"
)

type KubeCluster interface {
	runtime.Cluster

	Ingress() IngressClass
	PreparedClient() client.Prepared
}

type KubeClusterNamespace interface {
	runtime.ClusterNamespace

	KubeConfig() KubeConfig
}

type BackendProtocol string

const (
	BackendProtocol_HTTP  BackendProtocol = "http"
	BackendProtocol_GRPC  BackendProtocol = "grpc"
	BackendProtocol_GRPCS BackendProtocol = "grpcs"
)

type IngressClass interface {
	runtime.IngressClass

	Ensure(context.Context) ([]*schema.SerializedInvocation, error)
	Service() *IngressSelector
	Waiter(*rest.Config) KubeIngressWaiter
	Map(ctx context.Context, domain *schema.Domain, ns, name string) ([]*OpMapAddress, error)
	Annotate(ns, name string, domains []*schema.Domain, hasTLS bool, backendProtocol BackendProtocol, extensions []*anypb.Any) (*IngressAnnotations, error)
}

type IngressAnnotations struct {
	Annotations map[string]string
	Resources   []defs.MakeDefinition

	// If set, make sure that the ingress resource is created after the specified categories.
	SchedAfter []string
}

type KubeIngressWaiter interface {
	WaitUntilReady(ctx context.Context, ch chan *orchestration.Event) error
}

type IngressSelector struct {
	Namespace, ServiceName string
	ContainerPort          int
	PodSelector            map[string]string
}

type KubeConfig struct {
	Context     string // Only set if explicitly set in KubeEnv.
	Namespace   string
	Environment *schema.Environment
}

func InjectedKubeCluster(ctx context.Context) (KubeCluster, error) {
	c, err := execution.Get(ctx, runtime.ClusterInjection)
	if err != nil {
		return nil, err
	}

	if v, ok := c.(KubeCluster); ok {
		return v, nil
	}

	return nil, fnerrors.InternalError("expected a kubernetes cluster in context")
}

func InjectedKubeClusterNamespace(ctx context.Context) (KubeClusterNamespace, error) {
	c, err := execution.Get(ctx, runtime.ClusterNamespaceInjection)
	if err != nil {
		return nil, err
	}

	if v, ok := c.(KubeClusterNamespace); ok {
		return v, nil
	}

	return nil, fnerrors.InternalError("expected a kubernetes namespace in context")
}
