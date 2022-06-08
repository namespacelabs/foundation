// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"bytes"
	"context"
	"embed"
	"flag"
	"fmt"
	"path/filepath"
	"text/template"

	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

var (
	adminPort       = flag.Uint("admin_port", 19000, "Envoy admin port")
	xdsServerPort   = flag.Uint("xds_server_port", 18000, "Port that the Envoy controller is listening on")
	alsListenerPort = flag.Uint("als_listener_port", 18090, "gRPC Access Log Service (ALS) listener port")
	nodeCluster     = flag.String("node_cluster", "envoy_cluster", "Node cluster name")
	nodeID          = flag.String("node_id", "envoy_node", "Node Identifier")
)

//go:embed bootstrap-xds.yaml.tmpl
var embeddedBootstrapTmpl embed.FS

//go:embed grpcservicecrd.yaml
var embeddedGrpcServiceCrd embed.FS

type tmplData struct {
	AdminPort       uint32
	XDSServerPort   uint32
	ALSListenerPort uint32
	NodeCluster     string
	NodeId          string
}

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	configure.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	const (
		configVolume   = "fn--gateway-bootstrap"
		filename       = "boostrap-xds.yaml"
		gRPCServiceCrd = "GRPCService"
	)

	tmplData := tmplData{
		AdminPort:       uint32(*adminPort),
		XDSServerPort:   uint32(*xdsServerPort),
		ALSListenerPort: uint32(*alsListenerPort),
		NodeCluster:     *nodeCluster,
		NodeId:          *nodeID,
	}

	var bootstrapData bytes.Buffer

	t, err := template.ParseFS(embeddedBootstrapTmpl, "bootstrap-xds.yaml.tmpl")
	if err != nil {
		return fnerrors.InternalError("failed to parse bootstrap template: %w", err)
	}

	if err := t.Execute(&bootstrapData, tmplData); err != nil {
		return fnerrors.InternalError("failed to render bootstrap template: %w", err)
	}

	namespace := kubetool.FromRequest(req).Namespace

	// XXX use immutable ConfigMaps.
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Network Gateway ConfigMap",
		Resource:    "configmaps",
		Namespace:   namespace,
		Name:        configVolume,
		Body: corev1.ConfigMap(configVolume, namespace).WithData(
			map[string]string{
				filename: bootstrapData.String(),
			},
		),
	})

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Network Gateway gRPC Service CustomResourceDefinition",
		Resource:    "customresourcedefinitions",
		Namespace:   namespace,
		Name:        gRPCServiceCrd,
		Body:        embeddedGrpcServiceCrd,
	})

	serviceAccount := makeServiceAccount(req.Focus.Server)
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Network Gateway Service Account",
		Resource:    "serviceaccounts",
		Namespace:   namespace,
		Name:        serviceAccount,
		Body:        corev1.ServiceAccount(serviceAccount, namespace),
	})

	clusterRole := "fn-gateway-cluster-role"
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Network Gateway Cluster Role",
		Resource:    "clusterroles",
		Name:        clusterRole,
		Body: rbacv1.ClusterRole(clusterRole).WithRules(
			rbacv1.PolicyRule().WithAPIGroups("k8s.namespacelabs.dev").WithResources("grpcservices").WithVerbs("get", "list", "watch", "create", "update", "delete"),
		),
	})

	clusterRoleBinding := "fn-gateway-cluster-role-binding"
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Network Gateway Cluster Role Binding",
		Resource:    "clusterrolebindings",
		Name:        clusterRoleBinding,
		Body: rbacv1.ClusterRoleBinding(clusterRoleBinding).
			WithRoleRef(rbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(clusterRole)).
			WithSubjects(rbacv1.Subject().
				WithKind("ServiceAccount").
				WithNamespace(namespace).
				WithName(serviceAccount)),
	})

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
			Args: []string{"-c", filepath.Join("/config/", filename)},
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

func makeServiceAccount(srv *schema.Server) string {
	return fmt.Sprintf("fn-gateway-%s", kubedef.MakeDeploymentId(srv))
}
