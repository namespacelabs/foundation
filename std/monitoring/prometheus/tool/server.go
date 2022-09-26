// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"io/fs"
	"strings"

	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeblueprint"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
)

const (
	id                     = "prometheus.foundation.namespacelabs.dev"
	clusterRoleName        = "fn:prometheus"
	clusterRoleBindingName = "fn:prometheus"
	configMapName          = id
	promYaml               = "prometheus.yml"
)

var (
	volumeName = strings.Replace(id, ".", "-", -1)
)

type configureServer struct{}

func (configureServer) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	kr, err := kubetool.MustNamespace(r)
	if err != nil {
		return err
	}

	promYamlData, err := fs.ReadFile(embeddedData, promYaml)
	if err != nil {
		return err
	}

	g := kubeblueprint.GrantKubeACLs{
		DescriptionBase: "Prometheus",
		Rules: []*rbacv1.PolicyRuleApplyConfiguration{
			rbacv1.PolicyRule().
				WithAPIGroups("").
				WithResources("nodes", "nodes/proxy", "services", "endpoints", "pods").
				WithVerbs("get", "list", "watch"),
			rbacv1.PolicyRule().
				WithAPIGroups("extensions").
				WithResources("ingresses").
				WithVerbs("get", "list", "watch"),
			rbacv1.PolicyRule().
				WithNonResourceURLs("/metrics").
				WithVerbs("get"),
		},
	}

	if err := g.Compile(r, kubeblueprint.ClusterScope, out); err != nil {
		return err
	}

	configs := map[string]string{
		promYaml: string(promYamlData),
	}

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description:  "Prometheus ConfigMap",
		SetNamespace: kr.CanSetNamespace,
		Resource:     corev1.ConfigMap(configMapName, kr.Namespace).WithData(configs),
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			Volume: []*kubedef.SpecExtension_Volume{{
				Name: volumeName, // XXX generate unique names.
				VolumeType: &kubedef.SpecExtension_Volume_ConfigMap_{
					ConfigMap: &kubedef.SpecExtension_Volume_ConfigMap{
						Name: configMapName,
						Item: []*kubedef.SpecExtension_Volume_ConfigMap_Item{{
							Key:  promYaml,
							Path: promYaml,
						}},
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
				MountPath: "/etc/prometheus/",
			}},
		},
	})

	return nil
}

func (configureServer) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	kr, err := kubetool.MustNamespace(r)
	if err != nil {
		return err
	}

	out.Invocations = append(out.Invocations, kubedef.Delete{
		Description: "Prometheus ClusterRoleBinding",
		Resource:    "clusterrolebindings",
		Name:        clusterRoleBindingName,
	})

	out.Invocations = append(out.Invocations, kubedef.Delete{
		Description: "Prometheus ClusterRole",
		Resource:    "clusterroles",
		Name:        clusterRoleName,
	})

	out.Invocations = append(out.Invocations, kubedef.Delete{
		Description:  "Prometheus Service Account",
		Resource:     "serviceaccounts",
		Name:         makeServiceAccount(r.Focus.Server),
		SetNamespace: kr.CanSetNamespace,
		Namespace:    kr.Namespace,
	})

	return nil
}

func makeServiceAccount(srv runtime.Deployable) string {
	return kubedef.MakeDeploymentId(srv)
}
