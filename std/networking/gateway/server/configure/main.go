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
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeblueprint"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

var (
	adminPort = flag.Uint("admin_port", 19000, "Envoy admin port")
	debug     = flag.Bool("envoy_debug", false, "Sets the envoy log level to debug and "+
		"additionally enables the fine-grain logger with file level log control and runtime update "+
		"at administration interface.")
	xdsServerPort   = flag.Uint("xds_server_port", 18000, "Port that the Envoy controller is listening on")
	alsListenerPort = flag.Uint("als_listener_port", 18090, "gRPC Access Log Service (ALS) listener port")
	probePort       = flag.Uint("controller_health_probe_bind_port", 18081,
		"Kubernetes controller health probe probe binds to.")
	nodeCluster = flag.String("node_cluster", "envoy_cluster", "Node cluster name")
	nodeID      = flag.String("node_id", "envoy_node", "Node Identifier")
)

//go:embed bootstrap-xds.yaml.tmpl
var embeddedBootstrapTmpl embed.FS

//go:embed httpgrpctranscodercrd.yaml
var httpGrpcTranscoderCrd embed.FS

type tmplData struct {
	AdminPort       uint32
	XDSServerPort   uint32
	ALSListenerPort uint32
	NodeCluster     string
	NodeId          string
}

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	provisioning.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	const (
		configVolume = "fn--gateway-bootstrap"
		filename     = "boostrap-xds.yaml"
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

	kr, err := kubetool.FromRequest(req)
	if err != nil {
		return err
	}

	// XXX use immutable ConfigMaps.
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description:  "Network Gateway ConfigMap",
		SetNamespace: kr.CanSetNamespace,
		Resource: corev1.ConfigMap(configVolume, kr.Namespace).
			WithLabels(kubedef.ManagedByUs()).
			WithAnnotations(kubedef.BaseAnnotations()).
			WithData(
				map[string]string{
					filename: bootstrapData.String(),
				},
			),
	})

	body, err := httpGrpcTranscoderCrd.ReadFile("httpgrpctranscodercrd.yaml")
	if err != nil {
		return fnerrors.InternalError("failed to read the HTTP gRPC Transcoder CRD: %w", err)
	}

	apply, err := kubeparser.Single(body)
	if err != nil {
		return fnerrors.InternalError("failed to parse the HTTP gRPC Transcoder CRD: %w", err)
	}

	out.Invocations = append(out.Invocations, kubedef.Create{
		Description:      "Network Gateway HTTP gRPC Transcoder CustomResourceDefinition",
		Resource:         "customresourcedefinitions",
		Body:             apply.Resource,
		UpdateIfExisting: true,
	})

	grant := kubeblueprint.GrantKubeACLs{
		DescriptionBase: "Network Gateway",
	}

	grant.Rules = append(grant.Rules, rbacv1.PolicyRule().
		WithAPIGroups("k8s.namespacelabs.dev").
		WithResources("httpgrpctranscoders", "httpgrpctranscoders/status").
		WithVerbs("get", "list", "watch", "create", "update", "delete", "patch"))

	// We leverage `record.EventRecorder` from "k8s.io/client-go/tools/record" which
	// creates `Event` objects with the API group "". This rule ensures that
	// the event objects created by the controller are accepted by the k8s API server.
	grant.Rules = append(grant.Rules, rbacv1.PolicyRule().
		WithAPIGroups("").
		WithResources("events").
		WithVerbs("create"))

	if err := grant.Compile(req, kubeblueprint.NamespaceScope, out); err != nil {
		return err
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			Annotation: []*kubedef.SpecExtension_Annotation{
				{Key: "prometheus.io/scrape", Value: "true"},
				{Key: "prometheus.io/port", Value: fmt.Sprintf("%d", *adminPort)},
				{Key: "prometheus.io/path", Value: "/stats/prometheus"},
			},
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

	envoyArgs := []string{"-c", filepath.Join("/config/", filename)}

	// Envoy uses shared memory regions during hot restarts and this flag guarantees that
	// shared memory regions do not conflict when there are multiple running Envoy instances
	// on the same machine. See https://www.envoyproxy.io/docs/envoy/latest/operations/cli#cmdoption-use-dynamic-base-id.
	envoyArgs = append(envoyArgs, "--use-dynamic-base-id")

	if *debug {
		// https://www.envoyproxy.io/docs/envoy/latest/operations/cli#cmdoption-l
		envoyArgs = append(envoyArgs, "--log-level", "debug")
		// https://www.envoyproxy.io/docs/envoy/latest/operations/cli#cmdoption-enable-fine-grain-logging
		envoyArgs = append(envoyArgs, "--enable-fine-grain-logging")
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendContainer{
		With: &kubedef.ContainerExtension{
			Args: envoyArgs,
			// Mount the generated configuration under /config.
			VolumeMount: []*kubedef.ContainerExtension_VolumeMount{{
				Name:      configVolume,
				ReadOnly:  true,
				MountPath: "/config/",
			}},
			Probe: []*kubedef.ContainerExtension_Probe{
				// The controller's readyz will only become ready when it can connect to the http listener.
				{Kind: runtime.FnServiceReadyz, Path: "/readyz", ContainerPort: int32(*probePort)},
			},
		},
	})

	return nil
}

func (configuration) Delete(context.Context, provisioning.StackRequest, *provisioning.DeleteOutput) error {
	// XXX unimplemented
	return nil
}
