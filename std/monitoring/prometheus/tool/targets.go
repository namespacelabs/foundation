// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/schema"
)

type configureTargets struct{}

func (configureTargets) Apply(ctx context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
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

func (configureTargets) Delete(ctx context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	// Nothing to do, the annotations live with their corresponding servers.
	return nil
}
