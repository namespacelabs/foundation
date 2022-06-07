// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"path/filepath"

	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	configure.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req configure.StackRequest, out *configure.ApplyOutput) error {
	const (
		configVolume = "fn--gateway-bootstrap"
		filename     = "boostrap-xds.json"
	)

	namespace := kubetool.FromRequest(req).Namespace

	bootstrapData := "{}" // XXX TODO

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Network Gateway ConfigMap",
		Resource:    "configmaps",
		Namespace:   namespace,
		Name:        configVolume,
		Body: corev1.ConfigMap(configVolume, namespace).WithData(
			map[string]string{
				filename: bootstrapData,
			},
		),
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
