// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/rs/zerolog"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

const (
	id            = "grafana.foundation.namespacelabs.dev"
	configMapName = id
	mountPath     = "/etc/grafana"
)

var volumeName = strings.Replace(id, ".", "-", -1)

type tool struct{}

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	configure.Handle(h)
}

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	namespace := kubetool.FromRequest(r).Namespace

	configs := map[string]string{}
	items := []*kubedef.SpecExtension_Volume_ConfigMap_Item{}

	dashboard := "default-dashboard.json"

	data, err := fs.ReadFile(embeddedData, dashboard)
	if err != nil {
		return err
	}
	configs["dashboard"] = string(data)
	items = append(items, &kubedef.SpecExtension_Volume_ConfigMap_Item{
		Key:  "dashboard",
		Path: dashboard,
	})

	var b bytes.Buffer
	if err := iniTmpl.Execute(&b, iniTmplArgs{MountPath: mountPath, DashboardPath: dashboard}); err != nil {
		return fmt.Errorf("failed to generate configuration: %w", err)
	}

	grafanaIni := iniTmpl.Name()
	configs[grafanaIni] = b.String()
	items = append(items, &kubedef.SpecExtension_Volume_ConfigMap_Item{
		Key:  grafanaIni,
		Path: grafanaIni,
	})

	for _, endpoint := range r.Stack.GetEndpoint() {
		if !endpoint.HasKind("prometheus.io/endpoint") {
			continue
		}

		if endpoint.GetPort().GetContainerPort() <= 0 {
			zerolog.Ctx(ctx).Warn().
				Str("package_name", endpoint.GetServerOwner()).
				Msg("skipping endpoint, no container port")
			continue
		}

		host := endpoint.GetAllocatedName()
		port := endpoint.GetPort().GetContainerPort()

		var b bytes.Buffer
		if err := promTmpl.Execute(&b, promTmplArgs{Host: host, Port: port}); err != nil {
			return fmt.Errorf("failed to generate configuration: %w", err)
		}

		cfgName := fmt.Sprintf("%s.yml", host)
		configs[cfgName] = b.String()
		items = append(items, &kubedef.SpecExtension_Volume_ConfigMap_Item{
			Key:  cfgName,
			Path: fmt.Sprintf("provisioning/datasources/%s", cfgName),
		})
	}

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Grafana ConfigMap",
		Resource:    "configmaps",
		Namespace:   namespace,
		Name:        configMapName,
		Body:        corev1.ConfigMap(configMapName, namespace).WithData(configs),
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			Volume: []*kubedef.SpecExtension_Volume{{
				Name: volumeName, // XXX generate unique names.
				VolumeType: &kubedef.SpecExtension_Volume_ConfigMap_{
					ConfigMap: &kubedef.SpecExtension_Volume_ConfigMap{
						Name: configMapName,
						Item: items,
					},
				},
			}},
		},
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			VolumeMount: []*kubedef.ContainerExtension_VolumeMount{{
				Name:      volumeName,
				ReadOnly:  true,
				MountPath: mountPath,
			}},
		},
	})

	return nil
}

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	namespace := kubetool.FromRequest(r).Namespace

	out.Invocations = append(out.Invocations, kubedef.Delete{
		Description: "Grafana ConfigMap",
		Resource:    "configmaps",
		Namespace:   namespace,
		Name:        configMapName,
	})

	return nil
}

type promTmplArgs struct {
	Host string
	Port int32
}

type iniTmplArgs struct {
	MountPath     string
	DashboardPath string
}

var (
	promTmpl = template.Must(template.New("prometheus.yml").Parse(`apiVersion: 1

datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    orgId: 1
    url: http://{{.Host}}:{{.Port}}
    isDefault: true
    version: 1
    editable: false
`))

	iniTmpl = template.Must(template.New("grafana.ini").Parse(`[auth]
	disable_login_form = false
	[dashboards]
	default_home_dashboard_path = {{.MountPath}}/{{.DashboardPath}}
`))
)
