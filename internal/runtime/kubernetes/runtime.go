// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"
	"fmt"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubetool"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning/planninghooks"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var (
	ObserveInitContainerLogs = false

	kubernetesEnvConfigType      = cfg.DefineConfigType[*kubetool.KubernetesEnv]()
	kubernetesDeploymentPlanning = cfg.DefineConfigType[*client.DeploymentPlanning]()
)

const (
	DiscoveryStateKey  = "kubernetes.discovery"
	RestmapperStateKey = "kubernetes.restmapper"
)

type ProvideOverrideFunc func(context.Context, cfg.Configuration) (runtime.Class, error)

var classOverrides = map[string]ProvideOverrideFunc{}

func RegisterOverrideClass(name string, p ProvideOverrideFunc) {
	classOverrides[name] = p
}

func Register() {
	runtime.Register("kubernetes", func(ctx context.Context, cfg cfg.Configuration) (runtime.Class, error) {
		hostEnv, err := client.CheckGetHostEnv(cfg)
		if err != nil {
			return nil, err
		}

		fmt.Fprintf(console.Debug(ctx), "kubernetes: selected %+v for %q\n", hostEnv, cfg.EnvKey())
		tasks.TraceCaller(ctx, console.Debug, "kubernetes.New")

		if hostEnv.Provider != "" {
			if provider := classOverrides[hostEnv.Provider]; provider != nil {
				klass, err := provider(ctx, cfg)
				if err != nil {
					return nil, err
				}
				if klass != nil {
					return klass, nil
				}
			}
		}

		return kubernetesClass{}, nil
	})

	runtime.RegisterPrepare(DiscoveryStateKey, func(ctx context.Context, _ cfg.Configuration, cluster runtime.Cluster) (any, error) {
		unwrap, ok := cluster.(UnwrapCluster)
		if !ok {
			return nil, fnerrors.InternalError("expected kubernetes cluster")
		}

		kube := unwrap.KubernetesCluster()

		return client.NewDiscoveryClient(kube.RESTConfig(), kube.Prepared.Configuration.Ephemeral)
	})

	runtime.RegisterPrepare(RestmapperStateKey, func(ctx context.Context, _ cfg.Configuration, cluster runtime.Cluster) (any, error) {
		discoveryClient, err := cluster.EnsureState(ctx, DiscoveryStateKey)
		if err != nil {
			return nil, err
		}

		return restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient.(discovery.CachedDiscoveryInterface)), nil
	})

	planninghooks.RegisterPrepareHook("namespacelabs.dev/foundation/std/runtime/kubernetes.ApplyServerExtensions", prepareApplyServerExtensions)
}

type kubernetesClass struct{}

var _ runtime.Class = kubernetesClass{}

func (d kubernetesClass) AttachToCluster(ctx context.Context, cfg cfg.Configuration) (runtime.Cluster, error) {
	return ConnectToCluster(ctx, cfg)
}

func (d kubernetesClass) EnsureCluster(ctx context.Context, env cfg.Context, purpose string) (runtime.Cluster, error) {
	return ConnectToCluster(ctx, env.Configuration())
}

func (d kubernetesClass) Planner(ctx context.Context, cfg cfg.Context, purpose string) (runtime.Planner, error) {
	cluster, err := ConnectToCluster(ctx, cfg.Configuration())
	if err != nil {
		return nil, err
	}

	ingressClass, err := ingress.FromConfig(cluster.Prepared)
	if err != nil {
		return nil, err
	}

	planner, err := NewPlanner(ctx, cfg, cluster.FetchSystemInfo, ingressClass)
	if err != nil {
		return nil, err
	}

	planner.underlying = cluster
	return planner, nil
}

func bindNamespace(env cfg.Context) BoundNamespace {
	ns := ModuleNamespace(env.Workspace().Proto(), env.Environment())

	if conf, ok := kubernetesEnvConfigType.CheckGet(env.Configuration()); ok {
		ns = conf.Namespace
	}

	b := BoundNamespace{env: env.Environment(), namespace: ns}

	if conf, ok := kubernetesDeploymentPlanning.CheckGet(env.Configuration()); ok {
		b.planning = conf
	}

	return b
}

func MakeNamespace(env *schema.Environment, ns string) *applycorev1.NamespaceApplyConfiguration {
	return applycorev1.Namespace(ns).
		WithLabels(kubedef.MakeLabels(env, nil)).
		WithAnnotations(kubedef.MakeAnnotations(env, ""))
}

func PrepareProvisionWith(env *schema.Environment, ns string, systemInfo *kubedef.SystemInfo) (*rtypes.ProvisionProps, error) {
	// Ensure the namespace exist, before we go and apply definitions to it. Also, deployServer
	// assumes that a namespace already exists.
	def, err := (kubedef.Apply{
		Description: fmt.Sprintf("Namespace for %q", env.Name),
		Resource:    MakeNamespace(env, ns),
	}).ToDefinition()
	if err != nil {
		return nil, err
	}

	// Pass the computed namespace to the provisioning tool.
	return &rtypes.ProvisionProps{
		ProvisionInput: []rtypes.ProvisionInput{
			{Message: &kubetool.KubernetesEnv{Namespace: ns}},
			{Message: systemInfo},
		},
		Invocation: []*schema.SerializedInvocation{def},
	}, nil
}

func (r *Cluster) AttachTerminal(ctx context.Context, reference *runtimepb.ContainerReference, rio runtime.TerminalIO) error {
	cpr := &kubedef.ContainerPodReference{}
	if err := reference.Opaque.UnmarshalTo(cpr); err != nil {
		return fnerrors.InternalError("invalid reference: %w", err)
	}

	return r.attachTerminal(ctx, r.cli, cpr, rio)
}
