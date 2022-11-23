// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nscloud

import (
	"context"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeclient"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/universe/nscloud/configuration"
)

var clusterConfigType = cfg.DefineConfigType[*PrebuiltCluster]()

func RegisterClusterProvider() {
	client.RegisterConfigurationProvider("nscloud", provideCluster)
	kubernetes.RegisterOverrideClass("nscloud", provideClass)

	cfg.RegisterConfigurationProvider(func(cluster *configuration.Cluster) ([]proto.Message, error) {
		if cluster.ClusterId == "" {
			return nil, fnerrors.BadInputError("cluster_id must be specified")
		}

		messages := []proto.Message{
			&client.HostEnv{Provider: "nscloud"},
			&registry.Provider{Provider: "nscloud"},
			&PrebuiltCluster{ClusterId: cluster.ClusterId},
		}

		return messages, nil
	}, "foundation.providers.nscloud.config.Cluster")
}

func provideCluster(ctx context.Context, cfg cfg.Configuration) (client.ClusterConfiguration, error) {
	conf, ok := clusterConfigType.CheckGet(cfg)
	if !ok {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing configuration")
	}

	wres, err := api.WaitCluster(ctx, conf.ClusterId)
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	cluster := wres.Cluster

	var p client.ClusterConfiguration
	p.Ephemeral = conf.Ephemeral
	p.Config = *kubeclient.MakeApiConfig(&kubeclient.StaticConfig{
		EndpointAddress:          cluster.EndpointAddress,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
		ClientCertificateData:    cluster.ClientCertificateData,
		ClientKeyData:            cluster.ClientKeyData,
	})
	for _, lbl := range cluster.Label {
		p.Labels = append(p.Labels, &schema.Label{Name: lbl.Name, Value: lbl.Value})
	}
	return p, nil
}

func provideClass(ctx context.Context, cfg cfg.Configuration) (runtime.Class, error) {
	return runtimeClass{}, nil
}

type runtimeClass struct{}

var _ runtime.Class = runtimeClass{}

func (d runtimeClass) AttachToCluster(ctx context.Context, cfg cfg.Configuration) (runtime.Cluster, error) {
	conf, ok := clusterConfigType.CheckGet(cfg)
	if !ok {
		return nil, fnerrors.BadInputError("%s: no cluster configured", cfg.EnvKey())
	}

	cluster, err := api.GetCluster(ctx, conf.ClusterId)
	if err != nil {
		return nil, err
	}

	return d.ensureCluster(ctx, cfg, cluster.Cluster)
}

func (d runtimeClass) EnsureCluster(ctx context.Context, srv cfg.Configuration, purpose string) (runtime.Cluster, error) {
	if _, ok := clusterConfigType.CheckGet(srv); ok {
		return d.AttachToCluster(ctx, srv)
	}

	ephemeral := true
	result, err := api.CreateAndWaitCluster(ctx, "", ephemeral, purpose, nil)
	if err != nil {
		return nil, err
	}

	srv = srv.Derive(srv.EnvKey(), func(previous cfg.ConfigurationSlice) cfg.ConfigurationSlice {
		// Prepend to ensure that the prebuilt cluster is returned first.
		previous.Configuration = append(protos.WrapAnysOrDie(
			&PrebuiltCluster{ClusterId: result.ClusterId, Ephemeral: ephemeral},
		), previous.Configuration...)
		return previous
	})

	return d.ensureCluster(ctx, srv, result.Cluster)
}

func (d runtimeClass) ensureCluster(ctx context.Context, cfg cfg.Configuration, kc *api.KubernetesCluster) (runtime.Cluster, error) {
	// XXX This is confusing. We can call NewCluster because the runtime class
	// and cluster providers are registered with the same provider key. We
	// should instead create the cluster here, when the CreateCluster intent is
	// still clear.
	unbound, err := kubernetes.ConnectToCluster(ctx, cfg)
	if err != nil {
		return nil, err
	}

	unbound.FetchSystemInfo = func(ctx context.Context) (*kubedef.SystemInfo, error) {
		return &kubedef.SystemInfo{
			NodePlatform:         []string{"linux/amd64"},
			DetectedDistribution: "k3s",
		}, nil
	}

	return &cluster{cluster: unbound, config: kc}, nil
}

type cluster struct {
	cluster *kubernetes.Cluster
	config  *api.KubernetesCluster
}

var _ runtime.Cluster = &cluster{}
var _ kubedef.KubeCluster = &cluster{}

func (d *cluster) Class() runtime.Class {
	return runtimeClass{}
}

func (d *cluster) Bind(env cfg.Context) (runtime.ClusterNamespace, error) {
	bound, err := d.cluster.Bind(env)
	if err != nil {
		return nil, err
	}

	return clusterNamespace{ClusterNamespace: bound.(*kubernetes.ClusterNamespace), Config: d.config}, nil
}

func (d *cluster) Planner(env cfg.Context) runtime.Planner {
	base := kubernetes.NewPlanner(env, d.cluster.SystemInfo)

	return planner{Planner: base, config: d.config}
}

func (d *cluster) FetchDiagnostics(ctx context.Context, cr *runtimepb.ContainerReference) (*runtimepb.Diagnostics, error) {
	return d.cluster.FetchDiagnostics(ctx, cr)
}

func (d *cluster) FetchLogsTo(ctx context.Context, cr *runtimepb.ContainerReference, opts runtime.FetchLogsOpts, callback func(runtime.ContainerLogLine)) error {
	return d.cluster.FetchLogsTo(ctx, cr, opts, callback)
}

func (d *cluster) AttachTerminal(ctx context.Context, container *runtimepb.ContainerReference, io runtime.TerminalIO) error {
	return d.cluster.AttachTerminal(ctx, container, io)
}

func (d *cluster) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, notify runtime.PortForwardedFunc) (io.Closer, error) {
	return d.cluster.ForwardIngress(ctx, localAddrs, localPort, notify)
}

func (d *cluster) EnsureState(ctx context.Context, key string) (any, error) {
	return d.cluster.EnsureState(ctx, key)
}

func (d *cluster) DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error) {
	return d.cluster.DeleteAllRecursively(ctx, wait, progress)
}

func (d *cluster) PreparedClient() client.Prepared {
	return d.cluster.PreparedClient()
}

type planner struct {
	runtime.Planner
	config *api.KubernetesCluster
}

func (d planner) ComputeBaseNaming(source *schema.Naming) (*schema.ComputedNaming, error) {
	return &schema.ComputedNaming{
		Source:                   source,
		BaseDomain:               d.config.IngressDomain,
		TlsPassthroughBaseDomain: "int-" + d.config.IngressDomain, // XXX receive this value.
		Managed:                  schema.Domain_CLOUD_TERMINATION,
		UpstreamTlsTermination:   true,
		DomainFragmentSuffix:     d.config.ClusterId, // XXX fetch ingress external IP to calculate domain.
		UseShortAlias:            true,
	}, nil
}

type clusterNamespace struct {
	*kubernetes.ClusterNamespace
	Config *api.KubernetesCluster
}

var _ kubedef.KubeClusterNamespace = clusterNamespace{}

func (cr clusterNamespace) Planner() runtime.Planner {
	base := cr.ClusterNamespace.Planner()
	return planner{Planner: base, config: cr.Config}
}

func (cr clusterNamespace) DeleteRecursively(ctx context.Context, wait bool) (bool, error) {
	return cr.deleteCluster(ctx)
}

func (cr clusterNamespace) DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error) {
	return cr.deleteCluster(ctx)
}

func (cr clusterNamespace) deleteCluster(ctx context.Context) (bool, error) {
	if err := api.DestroyCluster(ctx, cr.Config.ClusterId); err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
