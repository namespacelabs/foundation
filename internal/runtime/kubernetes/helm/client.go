// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package helm

import (
	"context"
	"fmt"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewConfiguration(ctx context.Context, host kubedef.KubeCluster, namespace string) (*action.Configuration, error) {
	cfg := &action.Configuration{}

	// g := getter{cfg: client.NewClientConfig(ctx, host)}
	g := clusterWrapper{host}
	debugLogger := func(format string, v ...interface{}) {
		fmt.Fprintf(console.Debug(ctx), "helm: "+format+"\n", v...)
	}

	if err := cfg.Init(g, namespace, "", debugLogger); err != nil {
		return nil, err
	}

	return cfg, nil
}

func NewInstall(ctx context.Context, host kubedef.KubeCluster, releaseName, namespace string, chart *chart.Chart, values map[string]interface{}) (*release.Release, error) {
	return tasks.Return(ctx, tasks.Action("helm.install").Arg("chart", chart.Metadata.Name).Arg("name", releaseName),
		func(ctx context.Context) (*release.Release, error) {
			cfg, err := NewConfiguration(ctx, host, namespace)
			if err != nil {
				return nil, err
			}

			const dryRun = false

			install := action.NewInstall(cfg)
			install.ReleaseName = releaseName
			install.Namespace = namespace
			install.DryRun = dryRun
			install.ClientOnly = dryRun

			release, err := install.RunWithContext(ctx, chart, values)
			if err != nil {
				return nil, err
			}

			_ = tasks.Attachments(ctx).AttachSerializable("release.json", "helm-release", release)

			return release, nil
		})
}

type clusterWrapper struct {
	cfg kubedef.KubeCluster
}

func (g clusterWrapper) ToRESTConfig() (*rest.Config, error) {
	return g.cfg.PreparedClient().RESTConfig, nil
}

func (g clusterWrapper) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return client.NewDiscoveryClient(g.cfg.PreparedClient().RESTConfig, false)
}

func (g clusterWrapper) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (g clusterWrapper) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return g.cfg.PreparedClient().ClientConfig
}
