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
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"go.uber.org/atomic"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const machineEndpoint = "https://grpc-gateway-84umfjt8rm05f5dimftg.prod-metal.namespacelabs.nscloud.dev"
const registryAddr = "registry-fgfo23t6gn9jd834s36g.prod-metal.namespacelabs.nscloud.dev"

// const machineEndpoint = "https://grpc-gateway-84umfjt8rm05f5dimftg.prod-metal-c.namespacelabs.nscloud.dev"
// const registryAddr = "registry-fgfo23t6gn9jd834s36g.prod-metal-c.namespacelabs.nscloud.dev"

var (
	clusterCache = NewCache[*CreateClusterResult]()

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
	client.RegisterProvider("nscloud", provideCluster)
	client.RegisterDeferredProvider("nscloud", provideDeferred)
}

func provideCluster(ctx context.Context, env *schema.Environment, key *devhost.ConfigKey) (client.Provider, error) {
	conf := &PrebuiltCluster{}

	if key.Selector.Select(key.DevHost).Get(conf) {
		cluster, err := GetCluster(ctx, conf.ClusterId)
		if err != nil {
			return client.Provider{}, err
		}

		var p client.Provider
		p.ProviderSpecific = cluster
		p.Config = *makeConfig(cluster)
		return p, nil
	}

	cfg, err := CreateClusterForEnv(ctx, env, true)
	if err != nil {
		return client.Provider{}, err
	}

	return client.Provider{Config: *makeConfig(cfg.Cluster), ProviderSpecific: cfg.Cluster}, nil
}

type CreateClusterResult struct {
	ClusterId string
	Cluster   *KubernetesCluster
	Registry  *ImageRegistry
	Deadline  *time.Time
}

func CreateClusterForEnv(ctx context.Context, env *schema.Environment, ephemeral bool) (*CreateClusterResult, error) {
	if env == nil {
		return nil, fnerrors.InternalError("env is missing")
	}

	return clusterCache.Compute(env.Name, func() (*CreateClusterResult, error) {
		return CreateCluster(ctx, ephemeral, env.Name) // The environment name is the best we can do right now as a documented purpose.
	})
}

func CreateCluster(ctx context.Context, ephemeral bool, purpose string) (*CreateClusterResult, error) {
	ctx, done := context.WithTimeout(ctx, 15*time.Minute) // Wait for cluster creation up to 15 minutes.
	defer done()

	var cr *CreateKubernetesClusterResponse
	if err := tasks.Action("nscloud.cluster-create").Run(ctx, func(ctx context.Context) error {
		req := CreateKubernetesClusterRequest{
			Ephemeral:         ephemeral,
			DocumentedPurpose: purpose,
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

func makeConfig(cr *KubernetesCluster) *api.Config {
	cfg := api.NewConfig()
	cluster := api.NewCluster()
	cluster.CertificateAuthorityData = cr.CertificateAuthorityData
	cluster.Server = cr.EndpointAddress
	auth := api.NewAuthInfo()
	auth.ClientCertificateData = cr.ClientCertificateData
	auth.ClientKeyData = cr.ClientKeyData
	c := api.NewContext()
	c.Cluster = "default"
	c.AuthInfo = "default"

	cfg.Clusters["default"] = cluster
	cfg.AuthInfos["default"] = auth
	cfg.Contexts["default"] = c

	cfg.Kind = "Config"
	cfg.APIVersion = "v1"
	cfg.CurrentContext = "default"

	return cfg
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

func provideDeferred(ctx context.Context, ws *schema.Workspace, cfg *client.HostConfig) (runtime.DeferredRuntime, error) {
	conf := &PrebuiltCluster{}
	if !cfg.Selector.Select(cfg.DevHost).Get(conf) {
		compute.On(ctx).DetachWith(compute.Detach{
			Action: tasks.Action("nscloud.cluster-prepare").LogLevel(1),
			Do: func(ctx context.Context) error {
				// Kick off the cluster provisioning as soon as we can.
				_, _ = CreateClusterForEnv(ctx, cfg.Environment, true)
				return nil
			},
			BestEffort: true,
		})
	}

	return deferred{ws, cfg}, nil
}

type deferred struct {
	ws  *schema.Workspace
	cfg *client.HostConfig
}

var _ runtime.DeferredRuntime = deferred{}
var _ runtime.HasPrepareProvision = deferred{}
var _ runtime.HasTargetPlatforms = deferred{}

func (d deferred) New(ctx context.Context) (runtime.Runtime, error) {
	unbound, err := kubernetes.New(ctx, d.cfg.Environment, d.cfg.DevHost, devhost.ByEnvironment(d.cfg.Environment))
	if err != nil {
		return nil, err
	}

	p, err := unbound.Provider()
	if err != nil {
		return nil, err
	}

	if p.ProviderSpecific == nil {
		return nil, fnerrors.InternalError("cluster creation state is missing")
	}

	bound := unbound.Bind(d.ws, d.cfg.Environment)

	return clusterRuntime{Runtime: bound, Cluster: p.ProviderSpecific.(*KubernetesCluster)}, nil
}

func (d deferred) PrepareProvision(context.Context) (*rtypes.ProvisionProps, error) {
	// XXX fetch SystemInfo in the future.
	return kubernetes.PrepareProvisionWith(d.cfg.Environment, kubernetes.ModuleNamespace(d.ws, d.cfg.Environment), &kubedef.SystemInfo{
		NodePlatform:         []string{"linux/amd64"},
		DetectedDistribution: "k3s",
	})
}

func (d deferred) TargetPlatforms(context.Context) ([]specs.Platform, error) {
	// XXX fetch this in the future.
	p, err := devhost.ParsePlatform("linux/amd64")
	if err != nil {
		return nil, err
	}
	return []specs.Platform{p}, nil
}

type clusterRuntime struct {
	runtime.Runtime
	Cluster *KubernetesCluster
}

func (cr clusterRuntime) ComputeBaseNaming(ctx context.Context, source *schema.Naming) (*schema.ComputedNaming, error) {
	return &schema.ComputedNaming{
		Source:                  source,
		BaseDomain:              cr.Cluster.IngressDomain,
		Managed:                 schema.Domain_CLOUD_MANAGED,
		TlsFrontend:             true,
		TlsInclusterTermination: false,
		DomainFragmentSuffix:    "880g-" + cr.Cluster.ClusterId, // XXX fetch ingress external IP to calculate domain.
		UseShortAlias:           true,
	}, nil
}

func (cr clusterRuntime) DeleteRecursively(ctx context.Context, wait bool) (bool, error) {
	return cr.deleteCluster(ctx)
}

func (cr clusterRuntime) DeleteAllRecursively(ctx context.Context, wait bool, progress io.Writer) (bool, error) {
	return cr.deleteCluster(ctx)
}

func (cr clusterRuntime) deleteCluster(ctx context.Context) (bool, error) {
	if err := DestroyCluster(ctx, cr.Cluster.ClusterId); err != nil {
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
