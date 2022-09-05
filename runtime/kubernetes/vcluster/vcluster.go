// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package vcluster

import (
	"context"
	"fmt"
	"os"

	"github.com/loft-sh/vcluster/cmd/vclusterctl/cmd"
	"github.com/loft-sh/vcluster/cmd/vclusterctl/log"
	"github.com/loft-sh/vcluster/pkg/util/servicecidr"
	"helm.sh/helm/v3/pkg/chart"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/helm"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeobserver"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

var (
	chartRepo    = "https://charts.loft.sh"
	chartName    = "vcluster"
	chartVersion = "0.10.0"
	chartDigest  = schema.Digest{
		Algorithm: "sha256",
		Hex:       "e28f093c1be7ee78d2489dfa5d702be9ecc3f0d3a530e2951a6d554db1ea0ef2",
	}
)

type VCluster struct {
	kr          kubernetes.Unbound
	clusterName string
	namespace   string
	kubeconfig  *api.Config
}

func Create(env *schema.Environment, host *client.HostConfig, namespace string) compute.Computable[*VCluster] {
	return &vclusterCreation{
		env:       env,
		namespace: namespace,
		host:      host,
		chart:     helm.Chart(chartRepo, chartName, chartVersion, chartDigest),
	}
}

type vclusterCreation struct {
	env       *schema.Environment
	namespace string
	host      *client.HostConfig
	chart     compute.Computable[*chart.Chart]

	compute.LocalScoped[*VCluster]
}

var _ compute.Computable[*VCluster] = &vclusterCreation{}

func (vc *vclusterCreation) Action() *tasks.ActionEvent {
	return tasks.Action("vcluster.create").Arg("namespace", vc.namespace)
}

func (vc *vclusterCreation) Inputs() *compute.In {
	return compute.Inputs().Str("namespace", vc.namespace).Indigestible("host", vc.host).Computable("chart", vc.chart)
}

func (vc *vclusterCreation) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (vc *vclusterCreation) Compute(ctx context.Context, deps compute.Resolved) (*VCluster, error) {
	kr, err := kubernetes.NewFromConfig(ctx, vc.host)
	if err != nil {
		return nil, err
	}

	// XXX consolidate with kubernetes runtime.
	if _, err := kr.Client().CoreV1().Namespaces().Apply(ctx, kubernetes.MakeNamespace(vc.env, vc.namespace), kubedef.Ego()); err != nil {
		return nil, err
	}

	cidr := servicecidr.GetServiceCIDR(kr.Client(), vc.namespace)

	values := map[string]interface{}{
		"serviceCIDR": cidr,
		"syncer": map[string]interface{}{
			"extraArgs": []string{"--disable-sync-resources=ingresses"},
		},
		"storage": map[string]interface{}{
			"persistence": false,
		},
	}

	chart := compute.MustGetDepValue(deps, vc.chart, "chart")

	if _, err := helm.NewInstall(ctx, vc.host, vc.namespace, vc.namespace, chart, values); err != nil {
		return nil, err
	}

	// XXX add deadline.
	kubeConfig, err := tasks.Return(ctx, tasks.Action("vcluster.wait-for-deployment"), func(ctx context.Context) (*api.Config, error) {
		// Wait until one of the pods is available.
		return cmd.GetKubeConfig(ctx, kr.Client(), vc.namespace, vc.namespace, log.Discard)
	})
	if err != nil {
		return nil, err
	}

	return &VCluster{kr: kr, clusterName: vc.namespace, namespace: vc.namespace, kubeconfig: kubeConfig}, nil
}

func (vc *VCluster) Access(parentCtx context.Context) (*Connection, error) {
	return tasks.Return(parentCtx, tasks.Action("vcluster.access").Arg("name", vc.clusterName), func(parentCtx context.Context) (*Connection, error) {
		conn := &Connection{}

		ctx, cancel := context.WithCancel(parentCtx)
		conn.cancel = cancel

		p := kubeobserver.NewPodObserver(ctx, vc.kr.Client(), vc.namespace, map[string]string{
			"app":     "vcluster",
			"release": vc.clusterName,
		})

		errch := make(chan error, 1)
		portch := make(chan runtime.ForwardedPort)

		go func() {
			defer close(errch)
			defer close(portch)

			errch <- vc.kr.StartAndBlockPortFwd(ctx, kubernetes.StartAndBlockPortFwdArgs{
				Namespace:     vc.namespace,
				Identifier:    "vcluster",
				LocalAddrs:    []string{"127.0.0.1"},
				LocalPort:     0,
				ContainerPort: 8443,
				PodResolver:   p,
				ReportPorts: func(fp runtime.ForwardedPort) {
					portch <- fp
				},
			})
		}()

		conn, err := tasks.Return(ctx, tasks.Action("vcluster.wait-for-portfwd"), func(ctx context.Context) (*Connection, error) {
			// If the context is canceled, the port will be closed above.
			port, ok := <-portch
			if !ok {
				// Only way the channel was closed was if we saw an error.
				err, ok := <-errch
				if !ok {
					return nil, fnerrors.InternalError("unexpected error")
				}

				return nil, err
			}

			fmt.Fprintf(console.Debug(ctx), "vcluster: listening on port %d\n", port.LocalPort)

			tasks.Attachments(ctx).AddResult("local_port", port.LocalPort)

			kubeConfig := *vc.kubeconfig

			for key, original := range kubeConfig.Clusters {
				cluster := *original
				cluster.Server = fmt.Sprintf("https://127.0.0.1:%d", port.LocalPort)
				kubeConfig.Clusters[key] = &cluster
			}

			configFile, err := writeTempConfig(ctx, vc.clusterName, kubeConfig)
			if err != nil {
				conn.Close()
				return nil, err
			}

			fmt.Fprintf(console.Debug(ctx), "vcluster: wrote config to %s\n", configFile)

			conn.configFile = configFile

			return conn, nil
		})

		if err != nil {
			return nil, err
		}

		if err := conn.WaitUntilSystemReady(ctx); err != nil {
			conn.Close()
			return nil, err
		}

		return conn, nil
	})
}

type Connection struct {
	cancel     func()
	configFile string
}

func (c *Connection) Close() error {
	c.cancel()
	return nil
}

func (c *Connection) HostEnv() *client.HostEnv {
	return &client.HostEnv{
		Kubeconfig: c.configFile,
	}
}

func (c *Connection) Runtime(ctx context.Context) (kubernetes.Unbound, error) {
	return kubernetes.NewFromConfig(ctx, &client.HostConfig{
		Config:  planning.MakeConfigurationWith("vcluster", nil, nil),
		HostEnv: c.HostEnv(),
	})
}

func (c *Connection) WaitUntilSystemReady(ctx context.Context) error {
	cfg, err := client.NewRestConfigFromHostEnv(ctx, &client.HostConfig{HostEnv: c.HostEnv()})
	if err != nil {
		return err
	}

	w := kubeobserver.WaitOnResource{
		RestConfig:   cfg,
		Name:         "coredns",
		Namespace:    "kube-system",
		ResourceKind: "Deployment",
		ExpectedGen:  1,
	}

	return w.WaitUntilReady(ctx, nil)
}

func writeTempConfig(ctx context.Context, clusterName string, cfg api.Config) (string, error) {
	f, err := dirs.CreateUserTemp("vcluster", clusterName)
	if err != nil {
		return "", err
	}

	contents, err := clientcmd.Write(cfg)
	if err != nil {
		return "", err
	}

	if _, err := f.Write(contents); err != nil {
		return "", err
	}

	if err := f.Close(); err != nil {
		return "", err
	}

	fileName := f.Name()
	compute.On(ctx).Cleanup(tasks.Action("vcluster.remove-configuration"), func(ctx context.Context) error {
		return os.Remove(fileName)
	})

	return fileName, nil
}
