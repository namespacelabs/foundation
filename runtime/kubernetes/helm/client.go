// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package helm

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/release"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	diskcached "k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/workspace/dirs"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func NewConfiguration(ctx context.Context, host *client.HostConfig, namespace string) (*action.Configuration, error) {
	cfg := &action.Configuration{}

	g := getter{cfg: client.NewClientConfig(ctx, host)}
	debugLogger := func(format string, v ...interface{}) {
		fmt.Fprintf(console.Debug(ctx), "helm: "+format+"\n", v...)
	}

	if err := cfg.Init(g, namespace, "", debugLogger); err != nil {
		return nil, err
	}

	return cfg, nil
}

func NewInstall(ctx context.Context, host *client.HostConfig, releaseName, namespace string, chart *chart.Chart, values map[string]interface{}) (*release.Release, error) {
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

type getter struct {
	cfg clientcmd.ClientConfig
}

func (g getter) ToRESTConfig() (*rest.Config, error) {
	c, err := g.ToRawKubeConfigLoader().ClientConfig()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (g getter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	config, err := g.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	cacheDir, err := dirs.Ensure(dirs.Subdir("k8s-discovery"))
	if err != nil {
		return nil, err
	}

	httpCacheDir := filepath.Join(cacheDir, "http")
	discoveryCacheDir := computeDiscoverCacheDir(filepath.Join(cacheDir, "discovery"), config.Host)

	return diskcached.NewCachedDiscoveryClientForConfig(config, discoveryCacheDir, httpCacheDir, time.Duration(6*time.Hour))
}

func (g getter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient)
	return expander, nil
}

func (g getter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return g.cfg
}

// overlyCautiousIllegalFileCharacters matches characters that *might* not be supported.  Windows is really restrictive, so this is really restrictive
var overlyCautiousIllegalFileCharacters = regexp.MustCompile(`[^(\w/.)]`)

// computeDiscoverCacheDir takes the parentDir and the host and comes up with a "usually non-colliding" name.
func computeDiscoverCacheDir(parentDir, host string) string {
	// strip the optional scheme from host if its there:
	schemelessHost := strings.Replace(strings.Replace(host, "https://", "", 1), "http://", "", 1)
	// now do a simple collapse of non-AZ09 characters.  Collisions are possible but unlikely.  Even if we do collide the problem is short lived
	safeHost := overlyCautiousIllegalFileCharacters.ReplaceAllString(schemelessHost, "_")
	return filepath.Join(parentDir, safeHost)
}
