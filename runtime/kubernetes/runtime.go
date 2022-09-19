// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/types/known/anypb"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnerrors/stacktrace"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
)

var (
	ObserveInitContainerLogs = false
)

const RestmapperStateKey = "kubernetes.restmapper"

type ProvideOverrideFunc func(context.Context, planning.Configuration) (runtime.Class, error)

var classOverrides = map[string]ProvideOverrideFunc{}

func RegisterOverrideClass(name string, p ProvideOverrideFunc) {
	classOverrides[name] = p
}

func Register() {
	runtime.Register("kubernetes", func(ctx context.Context, cfg planning.Configuration) (runtime.Class, error) {
		hostEnv, err := client.CheckGetHostEnv(cfg)
		if err != nil {
			return nil, err
		}

		fmt.Fprintf(console.Debug(ctx), "kubernetes: selected %+v for %q\n", hostEnv, cfg.EnvKey())

		fmt.Fprintf(console.Debug(ctx), "%v\n", stacktrace.New())

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

	runtime.RegisterPrepare(RestmapperStateKey, func(ctx context.Context, _ planning.Configuration, cluster runtime.Cluster) (any, error) {
		kube, ok := cluster.(*Cluster)
		if !ok {
			return nil, fnerrors.InternalError("expected kubernetes cluster")
		}

		return client.NewRESTMapper(kube.RESTConfig(), kube.computedClient.Ephemeral())
	})

	frontend.RegisterPrepareHook("namespacelabs.dev/foundation/std/runtime/kubernetes.ApplyServerExtensions", prepareApplyServerExtensions)
}

type kubernetesClass struct{}

var _ runtime.Class = kubernetesClass{}

func (d kubernetesClass) AttachToCluster(ctx context.Context, cfg planning.Configuration) (runtime.Cluster, error) {
	return ConnectToCluster(ctx, cfg)
}

func (d kubernetesClass) EnsureCluster(ctx context.Context, cfg planning.Configuration) (runtime.Cluster, error) {
	return ConnectToCluster(ctx, cfg)
}

func newTarget(env planning.Context) clusterTarget {
	ns := ModuleNamespace(env.Workspace().Proto(), env.Environment())

	conf := &kubetool.KubernetesEnv{}
	if env.Configuration().Get(conf) {
		ns = conf.Namespace
	}

	return clusterTarget{env: env.Environment(), namespace: ns}
}

func MakeNamespace(env *schema.Environment, ns string) *applycorev1.NamespaceApplyConfiguration {
	return applycorev1.Namespace(ns).
		WithLabels(kubedef.MakeLabels(env, nil)).
		WithAnnotations(kubedef.MakeAnnotations(env, ""))
}

func PrepareProvisionWith(env *schema.Environment, ns string, systemInfo *kubedef.SystemInfo) (*rtypes.ProvisionProps, error) {
	packedHostEnv, err := anypb.New(&kubetool.KubernetesEnv{Namespace: ns})
	if err != nil {
		return nil, err
	}

	packedSystemInfo, err := anypb.New(systemInfo)
	if err != nil {
		return nil, err
	}

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
		ProvisionInput: []*anypb.Any{packedHostEnv, packedSystemInfo},
		Invocation:     []*schema.SerializedInvocation{def},
	}, nil
}

func (r *Cluster) AttachTerminal(ctx context.Context, reference *runtime.ContainerReference, rio runtime.TerminalIO) error {
	cpr := &kubedef.ContainerPodReference{}
	if err := reference.Opaque.UnmarshalTo(cpr); err != nil {
		return fnerrors.InternalError("invalid reference: %w", err)
	}

	return r.attachTerminal(ctx, r.cli, cpr, rio)
}
