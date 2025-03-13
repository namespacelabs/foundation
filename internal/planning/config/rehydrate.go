// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package config

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	envBinaryPb      = "config/env.binarypb"
	stackBinaryPb    = "config/stack.binarypb"
	ingressBinaryPb  = "config/ingress.binarypb"
	computedBinaryPb = "config/computed_configs.binarypb"
)

type Rehydrated struct {
	Env              *schema.Environment
	Stack            *schema.Stack
	IngressFragments []*schema.IngressFragment
	ComputedConfigs  *schema.ComputedConfigurations
}

func Rehydrate(ctx context.Context, srv planning.Server, imageID oci.ImageID) (*Rehydrated, error) {
	return tasks.Return(ctx, tasks.Action("rehydrate").Scope(srv.PackageName()).Str("ref", imageID), func(ctx context.Context) (*Rehydrated, error) {
		// XXX get a cluster's registry instead.
		reg, err := registry.GetRegistry(ctx, srv.SealedContext())
		if err != nil {
			return nil, err
		}

		ref, remoteOpts, err := oci.ParseRefAndKeychain(ctx, imageID.RepoAndDigest(), reg.Access())
		if err != nil {
			return nil, fnerrors.Newf("failed to parse: %w", err)
		}

		img, err := remote.Image(ref, remoteOpts...)
		if err != nil {
			return nil, fnerrors.InvocationError("registry", "failed to fetch config image: %w", err)
		}

		var r Rehydrated

		if err := oci.VisitFilesFromImage(img, func(layer, path string, typ byte, contents []byte) error {
			switch path {
			case envBinaryPb:
				r.Env = &schema.Environment{}
				if err := proto.Unmarshal(contents, r.Env); err != nil {
					return fnerrors.BadInputError("%s: failed to unmarshal: %w", path, err)
				}

			case stackBinaryPb:
				r.Stack = &schema.Stack{}
				if err := proto.Unmarshal(contents, r.Stack); err != nil {
					return fnerrors.BadInputError("%s: failed to unmarshal: %w", path, err)
				}

				for _, ep := range r.Stack.Endpoint {
					patchEndpoint(ep)
				}

			case ingressBinaryPb:
				list := &schema.IngressFragmentList{}
				if err := proto.Unmarshal(contents, list); err != nil {
					return fnerrors.BadInputError("%s: failed to unmarshal: %w", path, err)
				}

				for _, frag := range list.IngressFragment {
					patchEndpoint(frag.Endpoint)
				}

				r.IngressFragments = list.IngressFragment

			case computedBinaryPb:
				r.ComputedConfigs = &schema.ComputedConfigurations{}
				if err := proto.Unmarshal(contents, r.ComputedConfigs); err != nil {
					return fnerrors.BadInputError("%s: failed to unmarshal: %w", path, err)
				}
			}

			return nil
		}); err != nil {
			return nil, err
		}

		return &r, nil
	})
}

func patchEndpoint(ep *schema.Endpoint) {
	if ep == nil {
		return
	}

	if ep.DeprecatedPort != nil {
		ep.Ports = append(ep.Ports, &schema.Endpoint_PortMap{
			Port:         ep.DeprecatedPort,
			ExportedPort: ep.DeprecatedExportedPort,
		})
	}
}
