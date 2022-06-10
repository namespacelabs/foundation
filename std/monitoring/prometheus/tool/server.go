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
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
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
	namespace := kubetool.FromRequest(r).Namespace

	promYamlData, err := fs.ReadFile(embeddedData, promYaml)
	if err != nil {
		return err
	}

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Prometheus ClusterRole",
		Resource: rbacv1.ClusterRole(clusterRoleName).WithRules(
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
		),
	})

	serviceAccount := makeServiceAccount(r.Focus.Server)
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Prometheus ClusterRoleBinding",
		Resource: rbacv1.ClusterRoleBinding(clusterRoleBindingName).
			WithRoleRef(rbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(clusterRoleName)).
			WithSubjects(rbacv1.Subject().
				WithKind("ServiceAccount").
				WithNamespace(namespace).
				WithName(serviceAccount)),
	})

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Prometheus Service Account",
		Resource:    corev1.ServiceAccount(serviceAccount, namespace).WithLabels(map[string]string{}),
	})

	configs := map[string]string{
		promYaml: string(promYamlData),
	}

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Prometheus ConfigMap",
		Resource:    corev1.ConfigMap(configMapName, namespace).WithData(configs),
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			ServiceAccount: serviceAccount,
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
		Description: "Prometheus Service Account",
		Resource:    "serviceaccounts",
		Name:        makeServiceAccount(r.Focus.Server),
		Namespace:   kubetool.FromRequest(r).Namespace,
	})

	return nil
}

func makeServiceAccount(srv *schema.Server) string {
	return kubedef.MakeDeploymentId(srv)
}
