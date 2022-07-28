// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nscloud

import (
	"context"
	"encoding/json"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
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

var t *Cache[*CreateClusterResult]

func init() {
	t = NewCache[*CreateClusterResult]()
}

func RegisterClusterProvider() {
	client.RegisterProvider("nscloud", provideCluster)
	client.RegisterDeferredProvider("nscloud", provideDeferred)
}

func provideCluster(ctx context.Context, env *schema.Environment, key *devhost.ConfigKey) (client.Provider, error) {
	conf := &PrebuiltCluster{}

	if key.Selector.Select(key.DevHost).Get(conf) {
		var p client.Provider
		if err := json.Unmarshal(conf.SerializedConfig, &p.Config); err != nil {
			return p, err
		}
		return p, nil
	}

	cfg, err := CreateCluster(ctx, env, true)
	if err != nil {
		return client.Provider{}, err
	}

	return client.Provider{Config: *cfg.KubeConfig}, nil
}

type CreateClusterResult struct {
	ClusterId  string
	Cluster    *KubernetesCluster
	Registry   *ImageRegistry
	KubeConfig *api.Config
}

func CreateCluster(ctx context.Context, env *schema.Environment, ephemeral bool) (*CreateClusterResult, error) {
	if env == nil {
		return nil, fnerrors.InternalError("env is missing")
	}

	return t.Compute(env.Name, func() (*CreateClusterResult, error) {
		user, err := fnapi.LoadUser()
		if err != nil {
			return nil, err
		}

		var responses []CreateKubernetesClusterResponse

		if err := tasks.Action("nscloud.k8s-cluster.create").Arg("env", env.Name).Run(ctx, func(ctx context.Context) error {
			return fnapi.CallAPI(ctx, machineEndpoint, "nsl.vm.api.VMService/CreateKubernetesCluster", &CreateKubernetesClusterRequest{
				OpaqueUserAuth: user.Opaque,
			}, func(dec *json.Decoder) error {
				return dec.Decode(&responses)
			})
		}); err != nil {
			return nil, err
		}

		if len(responses) == 0 || responses[len(responses)-1].Status != "CLUSTER_READY" {
			return nil, fnerrors.InvocationError("failed to create cluster")
		}

		r := responses[len(responses)-1]

		tasks.Attachments(ctx).AddResult("cluster_id", r.ClusterId).AddResult("cluster_address", r.Cluster.EndpointAddress)

		if ephemeral {
			compute.On(ctx).Cleanup(tasks.Action("nscloud.k8s-cluster.cleanup"), func(ctx context.Context) error {
				return fnapi.CallAPI(ctx, machineEndpoint, "nsl.vm.api.VMService/DestroyKubernetesCluster", &DestroyKubernetesClusterRequest{
					OpaqueUserAuth: user.Opaque,
					ClusterId:      r.ClusterId,
				}, func(dec *json.Decoder) error {
					return nil
				})
			})
		}

		cfg := api.NewConfig()
		cluster := api.NewCluster()
		cluster.CertificateAuthorityData = r.Cluster.CertificateAuthorityData
		cluster.Server = r.Cluster.EndpointAddress
		auth := api.NewAuthInfo()
		auth.ClientCertificateData = r.Cluster.ClientCertificateData
		auth.ClientKeyData = r.Cluster.ClientKeyData
		c := api.NewContext()
		c.Cluster = "default"
		c.AuthInfo = "default"

		cfg.Clusters["default"] = cluster
		cfg.AuthInfos["default"] = auth
		cfg.Contexts["default"] = c

		cfg.Kind = "Config"
		cfg.APIVersion = "v1"
		cfg.CurrentContext = "default"

		if err := tasks.Action("nscloud.k8s-cluster.wait-for-node").Arg("env", env.Name).Run(ctx, func(ctx context.Context) error {
			clientCfg := clientcmd.NewDefaultClientConfig(*cfg, nil)
			restCfg, err := clientCfg.ClientConfig()
			if err != nil {
				return err
			}

			cli, err := k8s.NewForConfig(restCfg)
			if err != nil {
				return err
			}

			w, err := cli.CoreV1().Nodes().Watch(ctx, v1.ListOptions{})
			if err != nil {
				return err
			}

			defer w.Stop()

			// Wait until we see a node.
			for e := range w.ResultChan() {
				if e.Object != nil {
					return nil
				}
			}

			return nil
		}); err != nil {
			return nil, err
		}

		return &CreateClusterResult{
			ClusterId:  r.ClusterId,
			Cluster:    r.Cluster,
			Registry:   r.Registry,
			KubeConfig: cfg,
		}, nil
	})
}

func provideDeferred(ctx context.Context, ws *schema.Workspace, cfg *client.HostConfig) (runtime.DeferredRuntime, error) {
	conf := &PrebuiltCluster{}
	if !cfg.Selector.Select(cfg.DevHost).Get(conf) {
		compute.On(ctx).DetachWith(compute.Detach{
			Action: tasks.Action("nscloud.k8s-cluster.prepare"),
			Do: func(ctx context.Context) error {
				// Kick off the cluster provisioning as soon as we can.
				_, _ = CreateCluster(ctx, cfg.Environment, true)
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

	return unbound.Bind(d.ws, d.cfg.Environment), nil
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

type CreateKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
}

type CreateKubernetesClusterResponse struct {
	Status    string             `json:"status,omitempty"`
	ClusterId string             `json:"cluster_id,omitempty"`
	Cluster   *KubernetesCluster `json:"cluster,omitempty"`
	Registry  *ImageRegistry     `json:"registry,omitempty"`
}

type KubernetesCluster struct {
	EndpointAddress          string `json:"endpoint_address,omitempty"`
	CertificateAuthorityData []byte `json:"certificate_authority_data,omitempty"`
	ClientCertificateData    []byte `json:"client_certificate_data,omitempty"`
	ClientKeyData            []byte `json:"client_key_data,omitempty"`
}

type ImageRegistry struct {
	EndpointAddress string `json:"endpoint_address,omitempty"`
}

type DestroyKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
	ClusterId      string `json:"cluster_id,omitempty"`
}
