// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nscloud

import (
	"context"
	"encoding/json"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8s "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const machineEndpoint = "https://grpc-gateway-84umfjt8rm05f5dimftg.prod-metal.namespacelabs.nscloud.dev"

var t *Cache[client.Provider]

func init() {
	t = NewCache[client.Provider]()
}

func RegisterClusterProvider() {
	client.RegisterProvider("nscloud", provideCluster)
}

func provideCluster(ctx context.Context, env *schema.Environment, key *devhost.ConfigKey) (client.Provider, error) {
	if env == nil {
		return client.Provider{}, fnerrors.InternalError("env is missing")
	}

	return t.Compute(env.Name, func() (client.Provider, error) {
		user, err := fnapi.LoadUser()
		if err != nil {
			return client.Provider{}, err
		}

		var responses []CreateKubernetesClusterResponse

		if err := tasks.Action("nscloud.k8s-cluster.create").Arg("env", env.Name).Run(ctx, func(ctx context.Context) error {
			return fnapi.CallAPI(ctx, machineEndpoint, "nsl.vm.api.VMService/CreateKubernetesCluster", &CreateKubernetesClusterRequest{
				OpaqueUserAuth: user.Opaque,
			}, func(dec *json.Decoder) error {
				return dec.Decode(&responses)
			})
		}); err != nil {
			return client.Provider{}, err
		}

		if len(responses) == 0 || responses[len(responses)-1].Status != "CLUSTER_READY" {
			return client.Provider{}, fnerrors.InvocationError("failed to create cluster")
		}

		r := responses[len(responses)-1]

		tasks.Attachments(ctx).AddResult("cluster_id", r.Cluster.Id).AddResult("cluster_address", r.Cluster.EndpointAddress)

		compute.On(ctx).Cleanup(tasks.Action("nscloud.k8s-cluster.cleanup"), func(ctx context.Context) error {
			return fnapi.CallAPI(ctx, machineEndpoint, "nsl.vm.api.VMService/DestroyKubernetesCluster", &DestroyKubernetesClusterRequest{
				OpaqueUserAuth: user.Opaque,
				ClusterId:      r.Cluster.Id,
			}, func(dec *json.Decoder) error {
				return nil
			})
		})

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
			return client.Provider{}, err
		}

		return client.Provider{Config: *cfg}, nil
	})
}

type CreateKubernetesClusterRequest struct {
	OpaqueUserAuth []byte `json:"opaque_user_auth,omitempty"`
}

type CreateKubernetesClusterResponse struct {
	Status   string             `json:"status,omitempty"`
	Cluster  *KubernetesCluster `json:"cluster,omitempty"`
	Registry *ImageRegistry     `json:"registry,omitempty"`
}

type KubernetesCluster struct {
	Id                       string `json:"id,omitempty"`
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
