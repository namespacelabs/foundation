// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helm

import (
	"context"
	"fmt"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/std/tasks"
)

func NewConfiguration(ctx context.Context, host kubedef.KubeCluster, namespace string) (*action.Configuration, error) {
	cfg := &action.Configuration{}

	// g := getter{cfg: client.NewClientConfig(ctx, host)}
	g := clusterWrapper{host}

	log := console.TypedOutput(ctx, "helm", common.CatOutputTool)

	debugLogger := func(format string, v ...interface{}) {
		fmt.Fprintf(log, format+"\n", v...)
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

			upgrade := action.NewUpgrade(cfg)
			upgrade.Install = true
			upgrade.Namespace = namespace
			upgrade.DryRun = false
			upgrade.Atomic = true
			upgrade.Timeout = 5 * time.Minute
			upgrade.CleanupOnFail = true

			histClient := action.NewHistory(cfg)
			histClient.Max = 1

			var release *release.Release
			var releaseErr error
			if _, err := histClient.Run(releaseName); err == driver.ErrReleaseNotFound {
				install := action.NewInstall(cfg)
				install.ReleaseName = releaseName
				install.DryRun = upgrade.DryRun
				install.DisableHooks = upgrade.DisableHooks
				install.SkipCRDs = upgrade.SkipCRDs
				install.Timeout = upgrade.Timeout
				install.Wait = upgrade.Wait
				install.WaitForJobs = upgrade.WaitForJobs
				install.Devel = upgrade.Devel
				install.Namespace = upgrade.Namespace
				install.Atomic = upgrade.Atomic
				install.PostRenderer = upgrade.PostRenderer
				install.DisableOpenAPIValidation = upgrade.DisableOpenAPIValidation
				install.SubNotes = upgrade.SubNotes
				install.Description = upgrade.Description
				install.DependencyUpdate = upgrade.DependencyUpdate

				release, releaseErr = install.RunWithContext(ctx, chart, values)
			} else {
				release, releaseErr = upgrade.RunWithContext(ctx, releaseName, chart, values)
			}

			if releaseErr != nil {
				return nil, releaseErr
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
