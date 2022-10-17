// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/planning/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

type configureTargets struct{}

func (configureTargets) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	for _, endpoint := range r.Stack.GetInternalEndpoint() {
		for _, md := range endpoint.GetServiceMetadata() {
			if md.Kind != "prometheus.io/metrics" || md.Protocol != schema.HttpProtocol {
				continue
			}

			if endpoint.GetServerOwner() != r.Focus.Server.PackageName {
				// Only configure focus server
				continue
			}

			port := endpoint.Port
			if port.GetContainerPort() <= 0 {
				return fmt.Errorf("%s: no port specified", endpoint.ServerOwner)
			}

			var http *schema.HttpExportedService
			if md.Details != nil {
				http = &schema.HttpExportedService{}
				if err := md.Details.UnmarshalTo(http); err != nil {
					return fmt.Errorf("%s: failed to unmarshal http details", endpoint.ServerOwner)
				}
			}

			metricsPath := http.GetPath()
			if metricsPath == "" {
				metricsPath = "/metrics"
			}

			out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
				With: &kubedef.SpecExtension{
					Annotation: []*kubedef.SpecExtension_Annotation{
						{Key: "prometheus.io/scrape", Value: "true"},
						{Key: "prometheus.io/path", Value: metricsPath},
						{Key: "prometheus.io/port", Value: fmt.Sprintf("%d", port.ContainerPort)},
					},
				}})
		}
	}

	return nil
}

func (configureTargets) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	// Nothing to do, the annotations live with their corresponding servers.
	return nil
}
