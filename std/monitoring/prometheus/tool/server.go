// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/rs/zerolog"
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
	promServer             = "namespacelabs.dev/foundation/std/monitoring/prometheus/server"
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

	prom := r.Stack.GetServer(promServer)
	if prom == nil {
		return fmt.Errorf("%s: missing in the stack", promServer)
	}

	out.Definitions = append(out.Definitions, kubedef.Apply{
		Description: "Prometheus ClusterRole",
		Resource:    "clusterroles",
		Name:        clusterRoleName,
		Body: rbacv1.ClusterRole(clusterRoleName).WithRules(
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

	serviceAccount := makeServiceAccount(prom.Server)
	out.Definitions = append(out.Definitions, kubedef.Apply{
		Description: "Prometheus ClusterRoleBinding",
		Resource:    "clusterrolebindings",
		Name:        clusterRoleBindingName,
		Body: rbacv1.ClusterRoleBinding(clusterRoleBindingName).
			WithRoleRef(rbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(clusterRoleName)).
			WithSubjects(rbacv1.Subject().
				WithKind("ServiceAccount").
				WithNamespace(namespace).
				WithName(serviceAccount)),
	})

	out.Definitions = append(out.Definitions, kubedef.Apply{
		Description: "Prometheus Service Account",
		Resource:    "serviceaccounts",
		Namespace:   namespace,
		Name:        serviceAccount,
		Body:        corev1.ServiceAccount(serviceAccount, namespace).WithLabels(map[string]string{}),
	})

	configs := map[string]string{
		promYaml: string(promYamlData),
	}

	out.Definitions = append(out.Definitions, kubedef.Apply{
		Description: "Prometheus ConfigMap",
		Resource:    "configmaps",
		Namespace:   namespace,
		Name:        configMapName,
		Body:        corev1.ConfigMap(configMapName, namespace).WithData(configs),
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		For: promServer,
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
		For: promServer,
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
	prom := r.Stack.GetServer(promServer)
	if prom == nil {
		zerolog.Ctx(ctx).Warn().
			Msg("Nothing to do, prometheus not in the stack.")
		return nil
	}

	out.Ops = append(out.Ops, kubedef.Delete{
		Description: "Prometheus ClusterRoleBinding",
		Resource:    "clusterrolebindings",
		Name:        clusterRoleBindingName,
	})

	out.Ops = append(out.Ops, kubedef.Delete{
		Description: "Prometheus ClusterRole",
		Resource:    "clusterroles",
		Name:        clusterRoleName,
	})

	out.Ops = append(out.Ops, kubedef.Delete{
		Description: "Prometheus Service Account",
		Resource:    "serviceaccounts",
		Name:        makeServiceAccount(prom.Server),
		Namespace:   kubetool.FromRequest(r).Namespace,
	})

	return nil
}

func makeServiceAccount(srv *schema.Server) string {
	return kubedef.MakeDeploymentId(srv)
}
