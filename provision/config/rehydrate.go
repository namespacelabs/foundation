// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package config

import (
	"context"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
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

func Rehydrate(ctx context.Context, srv provision.Server, imageID oci.ImageID) (*Rehydrated, error) {
	reg, err := registry.GetRegistry(ctx, srv.Env())
	if err != nil {
		return nil, err
	}

	allocated, err := reg.AuthRepository(imageID)
	if err != nil {
		return nil, err
	}

	ref, remoteOpts, err := oci.ParseRefAndKeychain(ctx, allocated.ImageRef(), oci.ResolveOpts{Keychain: allocated.Keychain})
	if err != nil {
		return nil, err
	}

	img, err := remote.Image(ref, remoteOpts...)
	if err != nil {
		return nil, err
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

		case ingressBinaryPb:
			list := &schema.IngressFragmentList{}
			if err := proto.Unmarshal(contents, list); err != nil {
				return fnerrors.BadInputError("%s: failed to unmarshal: %w", path, err)
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
}
