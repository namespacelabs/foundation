// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package k3d

import (
	"context"
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/tcache"
	"namespacelabs.dev/foundation/std/cfg"
)

var confConfigType = cfg.DefineConfigType[*Configuration]()

type k3dRegistry struct {
	ContainerName      string
	PublicPort         string
	ContainerIPAddress string
}

var registries = tcache.NewCache[*k3dRegistry]()

var _ registry.Manager = &k3dRegistry{}
var _ buildkit.BuildkitAwareRegistry = &k3dRegistry{}

func Register() {
	registry.Register("k3d", func(ctx context.Context, ck cfg.Configuration) (registry.Manager, error) {
		conf, ok := confConfigType.CheckGet(ck)
		if !ok {
			return nil, fnerrors.BadInputError("can't use a k3d registry without a k3d configuration")
		}

		return registries.Compute(conf.RegistryContainerName, func() (*k3dRegistry, error) {
			client, err := docker.NewClient()
			if err != nil {
				return nil, err
			}

			inspected, err := client.ContainerInspect(ctx, conf.RegistryContainerName)
			if err != nil {
				return nil, fnerrors.InternalError("failed to inspect k3d registry: %w", err)
			}

			if inspected.NetworkSettings == nil || inspected.NetworkSettings.Networks["bridge"] == nil {
				return nil, fnerrors.InternalError("missing network configuration in k3d registry container")
			}

			if len(inspected.NetworkSettings.Ports["5000/tcp"]) == 0 {
				return nil, fnerrors.InternalError("missing port configuration in k3d registry container")
			}

			return &k3dRegistry{
				ContainerName:      conf.RegistryContainerName,
				PublicPort:         inspected.NetworkSettings.Ports["5000/tcp"][0].HostPort,
				ContainerIPAddress: inspected.NetworkSettings.Networks["bridge"].IPAddress,
			}, nil
		})
	})
}

func (r *k3dRegistry) IsInsecure() bool { return true }

func (r *k3dRegistry) baseUrl() string {
	return fmt.Sprintf("127.0.0.1:%s", r.PublicPort)
}

func (r *k3dRegistry) AllocateName(repository string) compute.Computable[oci.AllocatedName] {
	return registry.AllocateStaticName(r, r.baseUrl(), repository)
}

func (r *k3dRegistry) AttachKeychain(img oci.ImageID) (oci.AllocatedName, error) {
	return oci.AllocatedName{
		Parent:           r,
		InsecureRegistry: r.IsInsecure(),
		ImageID:          img,
	}, nil
}

func (r *k3dRegistry) CheckExportRequest(cli *buildkit.GatewayClient, name oci.AllocatedName) *buildkit.ExportToRegistryRequest {
	// There are some assumptions baked into this that are not verified at
	// runtime, notably that buildkit and the registry are deployed to the same
	// docker instance.
	if cli.UsesDocker() {
		trimmed := strings.TrimPrefix(name.Repository, r.baseUrl())
		if trimmed != name.Repository {
			return &buildkit.ExportToRegistryRequest{
				// Connect directly to the registry via the default bridge.
				Name:     fmt.Sprintf("%s:5000%s", r.ContainerIPAddress, trimmed),
				Insecure: r.IsInsecure(),
			}
		}
	}

	return nil
}
