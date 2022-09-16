// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"bytes"
	"context"
	"embed"
	"text/template"

	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeblueprint"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

//go:embed config.yaml.tmpl
var embeddedConfig embed.FS

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	configure.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	const (
		configVolume = "ns--k8s-event-exporter-config"
		filename     = "config.yaml"
	)

	var config bytes.Buffer

	t, err := template.ParseFS(embeddedConfig, "config.yaml.tmpl")
	if err != nil {
		return fnerrors.InternalError("failed to parse config template: %w", err)
	}

	if err := t.Execute(&config, nil); err != nil {
		return fnerrors.InternalError("failed to render config template: %w", err)
	}

	kr, err := kubetool.FromRequest(req)
	if err != nil {
		return err
	}

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description:  "Kubernetes Event Exporter ConfigMap",
		SetNamespace: kr.CanSetNamespace,
		Resource: corev1.ConfigMap(configVolume, kr.Namespace).WithData(
			map[string]string{
				filename: config.String(),
			},
		),
	})

	grant := kubeblueprint.GrantKubeACLs{
		DescriptionBase: "Kubernetes Event Exporter Namespace Scoped ACLs",
	}

	grant.Rules = append(grant.Rules, rbacv1.PolicyRule().
		WithAPIGroups("k8s.namespacelabs.dev", "", "apps").
		WithResources("deployments", "events", "pods", "replicasets").
		WithVerbs("get", "list", "watch"))

	if err := grant.Compile(req, kubeblueprint.NamespaceScope, out); err != nil {
		return err
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			Volume: []*kubedef.SpecExtension_Volume{{
				Name: configVolume,
				VolumeType: &kubedef.SpecExtension_Volume_ConfigMap_{
					ConfigMap: &kubedef.SpecExtension_Volume_ConfigMap{
						Name: configVolume,
						Item: []*kubedef.SpecExtension_Volume_ConfigMap_Item{{
							Key: filename, Path: filename, // Mount the config map key which matches the filename, into a local path also with the same filename.
						}},
					},
				},
			}},
		},
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			Args: []string{"-conf=/config/config.yaml"},
			// Mount the generated configuration under /config.
			VolumeMount: []*kubedef.ContainerExtension_VolumeMount{{
				Name:      configVolume,
				ReadOnly:  true,
				MountPath: "/config/",
			}},
		},
	})

	return nil
}

func (configuration) Delete(context.Context, configure.StackRequest, *configure.DeleteOutput) error {
	// XXX unimplemented
	return nil
}
