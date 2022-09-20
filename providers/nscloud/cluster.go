// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nscloud

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/bcicen/jstream"
	"github.com/dustin/go-humanize"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/providers/nscloud/config"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const machineEndpoint = "https://grpc-gateway-84umfjt8rm05f5dimftg.prod-metal.namespacelabs.nscloud.dev"
const registryAddr = "registry-fgfo23t6gn9jd834s36g.prod-metal.namespacelabs.nscloud.dev"

// const machineEndpoint = "https://grpc-gateway-84umfjt8rm05f5dimftg.prod-metal-c.namespacelabs.nscloud.dev"
// const registryAddr = "registry-fgfo23t6gn9jd834s36g.prod-metal-c.namespacelabs.nscloud.dev"

var (
	startCreateKubernetesCluster = fnapi.Call[CreateKubernetesClusterRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/StartCreateKubernetesCluster",
		PreAuthenticateRequest: func(user *fnapi.UserAuth, rt *CreateKubernetesClusterRequest) error {
			rt.OpaqueUserAuth = user.Opaque
			return nil
		},
	}

	getKubernetesCluster = fnapi.Call[GetKubernetesClusterRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/GetKubernetesCluster",
		PreAuthenticateRequest: func(user *fnapi.UserAuth, rt *GetKubernetesClusterRequest) error {
			rt.OpaqueUserAuth = user.Opaque
			return nil
		},
	}

	waitKubernetesCluster = fnapi.Call[WaitKubernetesClusterRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/WaitKubernetesCluster",
		PreAuthenticateRequest: func(user *fnapi.UserAuth, rt *WaitKubernetesClusterRequest) error {
			rt.OpaqueUserAuth = user.Opaque
			return nil
		},
	}

	listKubernetesClusters = fnapi.Call[ListKubernetesClustersRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/ListKubernetesClusters",
		PreAuthenticateRequest: func(ua *fnapi.UserAuth, rt *ListKubernetesClustersRequest) error {
			rt.OpaqueUserAuth = ua.Opaque
			return nil
		},
	}

	destroyKubernetesCluster = fnapi.Call[DestroyKubernetesClusterRequest]{
		Endpoint: machineEndpoint,
		Method:   "nsl.vm.api.VMService/DestroyKubernetesCluster",
		PreAuthenticateRequest: func(ua *fnapi.UserAuth, rt *DestroyKubernetesClusterRequest) error {
			rt.OpaqueUserAuth = ua.Opaque
			return nil
		},
	}
)

func RegisterClusterProvider() {
	client.RegisterConfigurationProvider("nscloud", provideCluster)
	kubernetes.RegisterOverrideClass("nscloud", provideClass)

	planning.RegisterConfigurationProvider(&config.Cluster{}, func(input *anypb.Any) ([]proto.Message, error) {
		cluster := &config.Cluster{}
		if err := input.UnmarshalTo(cluster); err != nil {
			return nil, err
		}

		if cluster.ClusterId == "" {
			return nil, fnerrors.BadInputError("cluster_id must be specified")
		}

		return []proto.Message{
			&client.HostEnv{Provider: "nscloud"},
			&registry.Provider{Provider: "nscloud"},
			&PrebuiltCluster{ClusterId: cluster.ClusterId},
		}, nil
	})
}

func getOrCreate(ctx context.Context, cfg planning.Configuration) (bool, *KubernetesCluster, error) {
	conf := &PrebuiltCluster{}

	if cfg.Get(conf) {
		cluster, err := GetCluster(ctx, conf.ClusterId)
		// Pre-defined clusters are never ephemeral.
		return false, cluster, err
	}

	result, err := CreateCluster(ctx, "", true, cfg.EnvKey()) // EnvKey is the best we can do re: purpose.
	if err != nil {
		return false, nil, err
	}

	return true, result.Cluster, nil
}

func provideCluster(ctx context.Context, cfg planning.Configuration) (client.ClusterConfiguration, error) {
	ephemeral, cluster, err := getOrCreate(ctx, cfg)
	if err != nil {
		return client.ClusterConfiguration{}, err
	}

	var p client.ClusterConfiguration
	p.Ephemeral = ephemeral
	p.ProviderSpecific = cluster
	p.Config = *client.MakeApiConfig(&client.StaticConfig{
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

type CreateClusterResult struct {
	ClusterId string
	Cluster   *KubernetesCluster
	Registry  *ImageRegistry
	Deadline  *time.Time
}

func CreateCluster(ctx context.Context, machineType string, ephemeral bool, purpose string) (*CreateClusterResult, error) {
	ctx, done := context.WithTimeout(ctx, 15*time.Minute) // Wait for cluster creation up to 15 minutes.
	defer done()

	var cr *CreateKubernetesClusterResponse
	if err := tasks.Action("nscloud.cluster-create").Run(ctx, func(ctx context.Context) error {
		req := CreateKubernetesClusterRequest{
			Ephemeral:         ephemeral,
			DocumentedPurpose: purpose,
			MachineType:       machineType,
		}

		if !environment.IsRunningInCI() {
			keys, err := UserSSHKeys()
			if err != nil {
				return err
			}

			if keys != nil {
				actualKeys, err := compute.GetValue(ctx, keys)
				if err != nil {
					return err
				}

				req.AuthorizedSshKeys = actualKeys
			}
		}

		var response StartCreateKubernetesClusterResponse
		if err := startCreateKubernetesCluster.Do(ctx, req, fnapi.DecodeJSONResponse(&response)); err != nil {
			return err
		}

		var progress clusterCreateProgress
		progress.status.Store("CREATE_ACCEPTED_WAITING_FOR_ALLOCATION")
		tasks.Attachments(ctx).SetProgress(&progress)

		clusterId := "<unknown>"
		lastStatus := "<none>"
		for cr == nil {
			if err := ctx.Err(); err != nil {
				return err // Check if we've been cancelled.
			}

			// We continue to wait for the cluster to become ready until we observe a READY.
			if err := waitKubernetesCluster.Do(ctx, WaitKubernetesClusterRequest{ClusterId: response.ClusterId}, func(body io.Reader) error {
				decoder := jstream.NewDecoder(body, 1)

				// jstream gives us the streamed array segmentation, however it
				// returns map[string]interface{} rather than typed objects. We
				// re-triggering parsing into the response type so the remainder
				// of our codebase operates on types.

				for mv := range decoder.Stream() {
					var resp CreateKubernetesClusterResponse
					if err := reparse(mv.Value, &resp); err != nil {
						return fnerrors.InvocationError("failed to parse response: %w", err)
					}

					progress.set(resp.Status)
					lastStatus = resp.Status

					if resp.ClusterId != "" {
						clusterId = resp.ClusterId
					}

					if resp.Status == "READY" {
						cr = &resp
						return nil
					}
				}

				return nil
			}); err != nil {
				return fnerrors.InvocationError("cluster never became ready (last status was %q, cluster id: %s): %w", lastStatus, clusterId, err)
			}
		}

		return nil
	}); err != nil {
		return nil, err
	}

	tasks.Attachments(ctx).
		AddResult("cluster_id", cr.ClusterId).
		AddResult("cluster_address", cr.Cluster.EndpointAddress).
		AddResult("deadline", cr.Cluster.Deadline)

	if shape := cr.Cluster.Shape; shape != nil {
		tasks.Attachments(ctx).
			AddResult("cluster_cpu", shape.VirtualCpu).
			AddResult("cluster_ram", humanize.IBytes(uint64(shape.MemoryMegabytes)*humanize.MiByte))
	}

	if ephemeral {
		compute.On(ctx).Cleanup(tasks.Action("nscloud.cluster-cleanup"), func(ctx context.Context) error {
			if err := DestroyCluster(ctx, cr.ClusterId); err != nil {
				// The cluster being gone is an acceptable state (it could have
				// been deleted by DeleteRecursively for example).
				if status.Code(err) == codes.NotFound {
					return nil
				}
			}

			return nil
		})
	}

	result := &CreateClusterResult{
		ClusterId: cr.ClusterId,
		Cluster:   cr.Cluster,
		Registry:  cr.Registry,
	}

	if cr.Deadline != "" {
		t, err := time.Parse(time.RFC3339, cr.Deadline)
		if err == nil {
			result.Deadline = &t
		}
	}

	return result, nil
}

func DestroyCluster(ctx context.Context, clusterId string) error {
	return destroyKubernetesCluster.Do(ctx, DestroyKubernetesClusterRequest{
		ClusterId: clusterId,
	}, nil)
}

func GetCluster(ctx context.Context, clusterId string) (*KubernetesCluster, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.get").Arg("id", clusterId), func(ctx context.Context) (*KubernetesCluster, error) {
		var response GetKubernetesClusterResponse
		if err := getKubernetesCluster.Do(ctx, GetKubernetesClusterRequest{ClusterId: clusterId}, fnapi.DecodeJSONResponse(&response)); err != nil {
			return nil, err
		}
		return response.Cluster, nil
	})
}

func ListClusters(ctx context.Context) (*KubernetesClusterList, error) {
	return tasks.Return(ctx, tasks.Action("nscloud.cluster-list"), func(ctx context.Context) (*KubernetesClusterList, error) {
		var list KubernetesClusterList
		if err := listKubernetesClusters.Do(ctx, ListKubernetesClustersRequest{}, fnapi.DecodeJSONResponse(&list)); err != nil {
			return nil, err
		}

		return &list, nil
	})
}

type clusterCreateProgress struct {
	status atomic.String
}

func (crp *clusterCreateProgress) set(status string)      { crp.status.Store(status) }
func (crp *clusterCreateProgress) FormatProgress() string { return crp.status.Load() }

func reparse(obj interface{}, target interface{}) error {
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, target)
}

func provideClass(ctx context.Context, cfg planning.Configuration) (runtime.Class, error) {
	conf := &PrebuiltCluster{}
	if !cfg.Get(conf) {
		compute.On(ctx).DetachWith(compute.Detach{
			Action: tasks.Action("nscloud.cluster-prepare").LogLevel(1),
			Do: func(ctx context.Context) error {
				// Kick off the cluster provisioning as soon as we can.
				// XXX this has to be a future, and we don't have the means for it yet.
				// _, _ = CreateCluster(ctx, "", true, cfg.EnvKey())
				return nil
			},
			BestEffort: true,
		})
	}

	return runtimeClass{}, nil
}

type runtimeClass struct{}

var _ runtime.Class = runtimeClass{}

func (d runtimeClass) AttachToCluster(ctx context.Context, cfg planning.Configuration) (runtime.Cluster, error) {
	conf := &PrebuiltCluster{}

	if !cfg.Get(conf) {
		return nil, fnerrors.BadInputError("%s: no cluster configured", cfg.EnvKey())
	}

	return d.EnsureCluster(ctx, cfg)
}

func (d runtimeClass) EnsureCluster(ctx context.Context, cfg planning.Configuration) (runtime.Cluster, error) {
	// XXX This is confusing. We can call NewCluster because the runtime class
	// and cluster providers are registered with the same provider key. We
	// should instead create the cluster here, when the CreateCluster intent is
	// still clear.
	unbound, err := kubernetes.ConnectToCluster(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &cluster{cluster: unbound}, nil
}

type cluster struct {
	cluster *kubernetes.Cluster
	kubernetes.ClusterAttachedState
}

var _ runtime.Cluster = &cluster{}
var _ kubedef.KubeCluster = &cluster{}

func (d *cluster) Class() runtime.Class {
	return runtimeClass{}
}

func (d *cluster) Bind(env planning.Context) (runtime.ClusterNamespace, error) {
	config, err := d.config()
	if err != nil {
		return nil, err
	}

	bound, err := d.cluster.Bind(env)
	if err != nil {
		return nil, err
	}

	return clusterNamespace{ClusterNamespace: bound.(*kubernetes.ClusterNamespace), Config: config}, nil
}

func (d *cluster) Planner(env planning.Context) runtime.Planner {
	base := kubernetes.NewPlanner(env, func(ctx context.Context) (*kubedef.SystemInfo, error) {
		return &kubedef.SystemInfo{
			NodePlatform:         []string{"linux/amd64"},
			DetectedDistribution: "k3s",
		}, nil
	})

	return planner{Planner: base, cluster: d, env: env.Environment(), workspace: env.Workspace().Proto()}
}

func (d *cluster) FetchDiagnostics(ctx context.Context, cr *runtime.ContainerReference) (*runtime.Diagnostics, error) {
	return d.cluster.FetchDiagnostics(ctx, cr)
}

func (d *cluster) FetchLogsTo(ctx context.Context, w io.Writer, cr *runtime.ContainerReference, opts runtime.FetchLogsOpts) error {
	return d.cluster.FetchLogsTo(ctx, w, cr, opts)
}

func (d *cluster) AttachTerminal(ctx context.Context, container *runtime.ContainerReference, io runtime.TerminalIO) error {
	return d.cluster.AttachTerminal(ctx, container, io)
}

func (d *cluster) ForwardIngress(ctx context.Context, localAddrs []string, localPort int, notify runtime.PortForwardedFunc) (io.Closer, error) {
	return d.cluster.ForwardIngress(ctx, localAddrs, localPort, notify)
}

func (d *cluster) EnsureState(ctx context.Context, key string) (any, error) {
	return d.ClusterAttachedState.EnsureState(ctx, key, d.cluster.HostConfig().Config, d, nil)
}

func (d *cluster) DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error) {
	return d.cluster.DeleteAllRecursively(ctx, wait, progress)
}

func (d *cluster) Client() *k8s.Clientset {
	return d.cluster.Client()
}

func (d *cluster) RESTConfig() *rest.Config {
	return d.cluster.RESTConfig()
}

func (d *cluster) ComputedConfig() clientcmd.ClientConfig {
	return d.cluster.ComputedConfig()
}

func (d *cluster) ClusterConfiguration() client.ClusterConfiguration {
	return d.cluster.ClusterConfiguration()
}

func (d *cluster) config() (*KubernetesCluster, error) {
	p := d.cluster.ClusterConfiguration()
	if p.ProviderSpecific == nil {
		return nil, fnerrors.InternalError("cluster creation state is missing")
	}

	return p.ProviderSpecific.(*KubernetesCluster), nil
}

type planner struct {
	kubernetes.Planner
	cluster   *cluster
	env       *schema.Environment
	workspace *schema.Workspace
}

func (d planner) ComputeBaseNaming(source *schema.Naming) (*schema.ComputedNaming, error) {
	config, err := d.cluster.config()
	if err != nil {
		return nil, err
	}

	return &schema.ComputedNaming{
		Source:                   source,
		BaseDomain:               config.IngressDomain,
		TlsPassthroughBaseDomain: "int-" + config.IngressDomain, // XXX receive this value.
		Managed:                  schema.Domain_CLOUD_TERMINATION,
		UpstreamTlsTermination:   true,
		DomainFragmentSuffix:     config.ClusterId, // XXX fetch ingress external IP to calculate domain.
		UseShortAlias:            true,
	}, nil
}

type clusterNamespace struct {
	*kubernetes.ClusterNamespace
	Config *KubernetesCluster
}

var _ kubedef.KubeClusterNamespace = clusterNamespace{}

func (cr clusterNamespace) DeleteRecursively(ctx context.Context, wait bool) (bool, error) {
	return cr.deleteCluster(ctx)
}

func (cr clusterNamespace) DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error) {
	return cr.deleteCluster(ctx)
}

func (cr clusterNamespace) deleteCluster(ctx context.Context) (bool, error) {
	if err := DestroyCluster(ctx, cr.Config.ClusterId); err != nil {
		if status.Code(err) == codes.NotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

type CreateKubernetesClusterRequest struct {
	OpaqueUserAuth    []byte   `json:"opaque_user_auth,omitempty"`
	Ephemeral         bool     `json:"ephemeral,omitempty"`
	DocumentedPurpose string   `json:"documented_purpose,omitempty"`
	AuthorizedSshKeys []string `json:"authorized_ssh_keys,omitempty"`
	MachineType       string   `json:"machine_type,omitempty"`
}

type GetKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	ClusterId      string `json:"cluster_id,omitempty"`
}

type WaitKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	ClusterId      string `json:"cluster_id,omitempty"`
}

type CreateKubernetesClusterResponse struct {
	Status    string             `json:"status,omitempty"`
	ClusterId string             `json:"cluster_id,omitempty"`
	Cluster   *KubernetesCluster `json:"cluster,omitempty"`
	Registry  *ImageRegistry     `json:"registry,omitempty"`
	Deadline  string             `json:"deadline,omitempty"`
}

type GetKubernetesClusterResponse struct {
	Cluster  *KubernetesCluster `json:"cluster,omitempty"`
	Registry *ImageRegistry     `json:"registry,omitempty"`
	Deadline string             `json:"deadline,omitempty"`
}

type StartCreateKubernetesClusterResponse struct {
	ClusterId string `json:"cluster_id,omitempty"`
}

type ListKubernetesClustersRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
}

type KubernetesClusterList struct {
	Clusters []KubernetesCluster `json:"cluster"`
}

type KubernetesCluster struct {
	ClusterId         string        `json:"cluster_id,omitempty"`
	Created           string        `json:"created,omitempty"`
	Deadline          string        `json:"deadline,omitempty"`
	SSHProxyEndpoint  string        `json:"ssh_proxy_endpoint,omitempty"`
	SshPrivateKey     []byte        `json:"ssh_private_key,omitempty"`
	DocumentedPurpose string        `json:"documented_purpose,omitempty"`
	Shape             *ClusterShape `json:"shape,omitempty"`

	EndpointAddress          string `json:"endpoint_address,omitempty"`
	CertificateAuthorityData []byte `json:"certificate_authority_data,omitempty"`
	ClientCertificateData    []byte `json:"client_certificate_data,omitempty"`
	ClientKeyData            []byte `json:"client_key_data,omitempty"`

	KubernetesDistribution string   `json:"kubernetes_distribution,omitempty"`
	Platform               []string `json:"platform,omitempty"`

	IngressDomain string `json:"ingress_domain,omitempty"`

	Label []*LabelEntry `json:"label,omitempty"`
}

type ImageRegistry struct {
	EndpointAddress string `json:"endpoint_address,omitempty"`
}

type ClusterShape struct {
	VirtualCpu      int32 `json:"virtual_cpu,omitempty"`
	MemoryMegabytes int32 `json:"memory_megabytes,omitempty"`
}

type DestroyKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	ClusterId      string `json:"cluster_id,omitempty"`
}

type LabelEntry struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}
