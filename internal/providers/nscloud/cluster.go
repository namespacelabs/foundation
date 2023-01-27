// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nscloud

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/pflag"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	k8sapi "k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeclient"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress/nginx"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/universe/nscloud/configuration"
)

var (
	rpcEndpointOverride string
	regionName          string
	defaultMachineType  string
)

var (
	clusterConfigType = cfg.DefineConfigType[*PrebuiltCluster]()
)

func SetupFlags(flags *pflag.FlagSet) {
	flags.StringVar(&rpcEndpointOverride, "nscloud_endpoint", "", "Where to dial to when reaching nscloud.")
	_ = flags.MarkHidden("nscloud_endpoint")
	flags.StringVar(&regionName, "nscloud_region", "fra1", "Which region to use.")
	_ = flags.MarkHidden("nscloud_region")
	flags.StringVar(&defaultMachineType, "nscloud_default_machine_type", "", "If specified, overrides the default machine type new clusters are created with.")
	_ = flags.MarkHidden("nscloud_default_machine_type")
}

func Register() {
	rpcEndpoint := rpcEndpointOverride
	if rpcEndpoint == "" {
		rpcEndpoint = fmt.Sprintf("https://api.%s.nscluster.cloud", regionName)
	}

	api.Endpoint = api.MakeAPI(rpcEndpoint)

	RegisterRegistry()
	RegisterClusterProvider()
}

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

func MakeConfig(cluster *api.KubernetesCluster) k8sapi.Config {
	return *kubeclient.MakeApiConfig(&kubeclient.StaticConfig{
		EndpointAddress:          cluster.EndpointAddress,
		CertificateAuthorityData: cluster.CertificateAuthorityData,
		ClientCertificateData:    cluster.ClientCertificateData,
		ClientKeyData:            cluster.ClientKeyData,
	})
}

func provideCluster(ctx context.Context, cfg cfg.Configuration) (client.ClusterConfiguration, error) {
	conf, ok := clusterConfigType.CheckGet(cfg)
	if !ok {
		return client.ClusterConfiguration{}, fnerrors.InternalError("missing configuration")
	}

	return provideClusterExt(ctx, conf.ClusterId, conf.Ephemeral)
}

func provideClusterExt(ctx context.Context, clusterId string, ephemeral bool) (client.ClusterConfiguration, error) {
	wres, err := api.WaitCluster(ctx, api.Endpoint, clusterId)
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	cluster := wres.Cluster

	var p client.ClusterConfiguration
	p.Ephemeral = ephemeral
	p.Config = MakeConfig(cluster)
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

	response, err := api.GetCluster(ctx, api.Endpoint, conf.ClusterId)
	if err != nil {
		return nil, err
	}

	return ensureCluster(ctx, cfg, response.Cluster.ClusterId, response.Cluster.IngressDomain, response.Registry, false)
}

func (d runtimeClass) EnsureCluster(ctx context.Context, env cfg.Context, purpose string) (runtime.Cluster, error) {
	config := env.Configuration()
	if _, ok := clusterConfigType.CheckGet(config); ok {
		cluster, err := d.AttachToCluster(ctx, config)
		return cluster, err
	}

	ephemeral := env.Environment().Ephemeral
	response, err := api.CreateCluster(ctx, api.Endpoint, api.CreateClusterOpts{MachineType: defaultMachineType, Ephemeral: ephemeral, Purpose: purpose})
	if err != nil {
		return nil, err
	}

	return ensureCluster(ctx, config, response.ClusterId, response.ClusterFragment.IngressDomain, response.Registry, ephemeral)
}

func (d runtimeClass) Planner(ctx context.Context, env cfg.Context, purpose string) (runtime.Planner, error) {
	if conf, ok := clusterConfigType.CheckGet(env.Configuration()); ok {
		response, err := api.GetCluster(ctx, api.Endpoint, conf.ClusterId)
		if err != nil {
			return nil, err
		}

		return completePlanner(ctx, env, conf.ClusterId, response.Cluster.IngressDomain, response.Registry, false)
	}

	response, err := api.CreateCluster(ctx, api.Endpoint, api.CreateClusterOpts{Ephemeral: true, Purpose: purpose})
	if err != nil {
		return nil, err
	}

	return completePlanner(ctx, env, response.ClusterId, response.ClusterFragment.IngressDomain, response.Registry, env.Environment().Ephemeral)
}

func completePlanner(ctx context.Context, env cfg.Context, clusterId, ingressDomain string, registry *api.ImageRegistry, ephemeral bool) (planner, error) {
	base := kubernetes.NewPlannerWithRegistry(env, nscloudRegistry{registry: registry, clusterID: clusterId},
		func(ctx context.Context) (*kubedef.SystemInfo, error) {
			return &kubedef.SystemInfo{
				NodePlatform:         []string{"linux/amd64"},
				DetectedDistribution: "k3s",
			}, nil
		}, nginx.Ingress())

	return planner{Planner: base, clusterId: clusterId, ingressDomain: ingressDomain, registry: registry, ephemeral: ephemeral}, nil
}

func ensureCluster(ctx context.Context, cfg cfg.Configuration, clusterId, ingressDomain string, registry *api.ImageRegistry, ephemeral bool) (*cluster, error) {
	result, err := provideClusterExt(ctx, clusterId, ephemeral)
	if err != nil {
		return nil, err
	}

	// The generated configuration is in `default`.
	cli, err := client.NewClientFromResult(ctx, &client.HostEnv{Context: "default"}, result.AsResult())
	if err != nil {
		return nil, err
	}

	unbound, err := kubernetes.NewCluster(cli, cfg, kubernetes.NewClusterOpts{
		FetchSystemInfo: func(ctx context.Context) (*kubedef.SystemInfo, error) {
			return &kubedef.SystemInfo{
				NodePlatform:         []string{"linux/amd64"},
				DetectedDistribution: "k3s",
			}, nil
		},
	})
	if err != nil {
		return nil, err
	}

	return &cluster{cluster: unbound, clusterId: clusterId, ingressDomain: ingressDomain, registry: registry}, nil
}

func newIngress(cfg cfg.Configuration, clusterId, ingressDomain string) kubedef.IngressClass {
	return ingressClass{IngressClass: nginx.Ingress(), ingressDomain: ingressDomain, clusterId: clusterId}
}

type cluster struct {
	cluster       *kubernetes.Cluster
	clusterId     string
	ingressDomain string
	registry      *api.ImageRegistry
}

var _ runtime.Cluster = &cluster{}
var _ kubedef.KubeCluster = &cluster{}
var _ kubernetes.UnwrapCluster = &cluster{}

func (d *cluster) KubernetesCluster() *kubernetes.Cluster {
	return d.cluster
}

func (d *cluster) Class() runtime.Class {
	return runtimeClass{}
}

func (d *cluster) Ingress() kubedef.IngressClass {
	return newIngress(d.cluster.Configuration, d.clusterId, d.ingressDomain)
}

func (d *cluster) Bind(ctx context.Context, env cfg.Context) (runtime.ClusterNamespace, error) {
	planner, err := completePlanner(ctx, env, d.clusterId, d.ingressDomain, d.registry, env.Environment().Ephemeral)
	if err != nil {
		return nil, err
	}

	return clusterNamespaceFromPlanner(planner, d), nil
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
	// It's important that we don't defer to d.cluster.EnsureState() as it would
	// then pass `d.cluster` as `runtime.Cluster`, rather than `d`.
	return d.cluster.ClusterAttachedState.EnsureState(ctx, key, d.cluster.Configuration, d, nil)
}

func (d *cluster) EnsureKeyedState(ctx context.Context, key, secondary string) (any, error) {
	// It's important that we don't defer to d.cluster.EnsureState() as it would
	// then pass `d.cluster` as `runtime.Cluster`, rather than `d`.
	return d.cluster.ClusterAttachedState.EnsureState(ctx, key, d.cluster.Configuration, d, &secondary)
}

func (d *cluster) DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error) {
	return d.cluster.DeleteAllRecursively(ctx, wait, progress)
}

func (d *cluster) PreparedClient() client.Prepared {
	return d.cluster.PreparedClient()
}

type ingressClass struct {
	kubedef.IngressClass

	ingressDomain string
	clusterId     string
}

func (d ingressClass) ComputeNaming(_ *schema.Environment, source *schema.Naming) (*schema.ComputedNaming, error) {
	return &schema.ComputedNaming{
		Source:                   source,
		BaseDomain:               d.ingressDomain,
		TlsPassthroughBaseDomain: "int-" + d.ingressDomain, // XXX receive this value.
		Managed:                  schema.Domain_CLOUD_TERMINATION,
		UpstreamTlsTermination:   true,
		DomainFragmentSuffix:     d.clusterId, // XXX fetch ingress external IP to calculate domain.
		UseShortAlias:            true,
	}, nil
}

func (d ingressClass) PrepareRoute(ctx context.Context, _ *schema.Environment, _ *schema.Stack_Entry, domain *schema.Domain, ns, name string) (*kubedef.IngressAllocatedRoute, error) {
	return nil, nil
}

type planner struct {
	kubernetes.Planner
	clusterId     string
	ingressDomain string
	registry      *api.ImageRegistry
	ephemeral     bool
}

func (d planner) Ingress() runtime.IngressClass {
	return newIngress(d.Configuration, d.clusterId, d.ingressDomain)
}

func (d planner) EnsureClusterNamespace(ctx context.Context) (runtime.ClusterNamespace, error) {
	cluster, err := ensureCluster(ctx, d.Planner.Configuration, d.clusterId, d.ingressDomain, d.registry, d.ephemeral)
	if err != nil {
		return nil, err
	}

	return clusterNamespaceFromPlanner(d, cluster), nil
}

func clusterNamespaceFromPlanner(d planner, cluster *cluster) clusterNamespace {
	return clusterNamespace{
		ClusterNamespace: d.Planner.ClusterNamespaceFor(cluster, cluster.cluster),
		clusterId:        d.clusterId,
	}
}

type clusterNamespace struct {
	*kubernetes.ClusterNamespace
	clusterId string
}

var _ kubedef.KubeClusterNamespace = clusterNamespace{}

func (cr clusterNamespace) DeleteRecursively(ctx context.Context, wait bool) (bool, error) {
	return cr.deleteCluster(ctx)
}

func (cr clusterNamespace) DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error) {
	return cr.deleteCluster(ctx)
}

func (cr clusterNamespace) deleteCluster(ctx context.Context) (bool, error) {
	if err := api.DestroyCluster(ctx, api.Endpoint, cr.clusterId); err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
