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
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/helm"
	"namespacelabs.dev/foundation/schema"
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
}

func Create(ctx context.Context, host *client.HostConfig, namespace string) (*VCluster, error) {
	return tasks.Return(ctx, tasks.Action("vcluster.create").Arg("namespace", namespace),
		func(ctx context.Context) (*VCluster, error) {
			chart, err := compute.GetValue(ctx, helm.Chart(chartRepo, chartName, chartVersion, chartDigest))
			if err != nil {
				return nil, err
			}

			kr, err := kubernetes.NewFromConfig(ctx, host)
			if err != nil {
				return nil, err
			}

			cidr := servicecidr.GetServiceCIDR(kr.Client(), namespace)

			values := map[string]interface{}{
				"serviceCIDR": cidr,
				"syncer": map[string]interface{}{
					"extraArgs": []string{"--disable-sync-resources=ingresses"},
				},
				"storage": map[string]interface{}{
					"persistence": false,
				},
			}

			if _, err := helm.NewInstall(ctx, host, namespace, namespace, chart, values); err != nil {
				return nil, err
			}

			return &VCluster{kr: kr, clusterName: namespace, namespace: namespace}, nil
		})
}

func (vc *VCluster) Access(parentCtx context.Context) (*Connection, error) {
	return tasks.Return(parentCtx, tasks.Action("vcluster.access").Arg("name", vc.clusterName), func(parentCtx context.Context) (*Connection, error) {
		conn := &Connection{}

		p := kubernetes.NewPodObserver(vc.kr.Client(), vc.namespace, map[string]string{
			"app":     "vcluster",
			"release": vc.clusterName,
		})

		ctx, cancel := context.WithCancel(parentCtx)
		conn.cancel = cancel

		p.Start(ctx)

		// XXX add deadline.
		kubeConfig, err := tasks.Return(ctx, tasks.Action("vcluster.wait-for-deployment"), func(ctx context.Context) (*api.Config, error) {
			// Wait until one of the pods is available.
			// _, err := p.Wait(ctx)
			return cmd.GetKubeConfig(ctx, vc.kr.Client(), vc.clusterName, vc.namespace, log.Discard)
		})
		if err != nil {
			cancel()
			return nil, err
		}

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

				Watch: func(ctx context.Context, f func(*v1.Pod, int64, error)) func() {
					return p.Watch(f)
				},

				ReportPorts: func(fp runtime.ForwardedPort) {
					portch <- fp
				},
			})
		}()

		return tasks.Return(ctx, tasks.Action("vcluster.wait-for-portfwd"), func(ctx context.Context) (*Connection, error) {
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

			for _, cluster := range kubeConfig.Clusters {
				cluster.Server = fmt.Sprintf("https://127.0.0.1:%d", port.LocalPort)
			}

			configFile, err := writeTempConfig(ctx, vc.clusterName, *kubeConfig)
			if err != nil {
				conn.Close()
				return nil, err
			}

			fmt.Fprintf(console.Debug(ctx), "vcluster: wrote config to %s\n", configFile)

			conn.configFile = configFile

			return conn, nil
		})
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

func (c *Connection) Runtime(ctx context.Context) (kubernetes.Unbound, error) {
	return kubernetes.NewFromConfig(ctx, &client.HostConfig{
		DevHost:  nil,
		Selector: nil,
		HostEnv: &client.HostEnv{
			Kubeconfig: c.configFile,
		},
	})
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
