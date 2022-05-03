// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package config

import (
	"context"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
)

const (
	envBinaryPb     = "config/env.binarypb"
	stackBinaryPb   = "config/stack.binarypb"
	ingressBinaryPb = "config/ingress.binarypb"
)

type Rehydrated struct {
	Env              *schema.Environment
	Stack            *schema.Stack
	IngressFragments []*schema.IngressFragment
}

func Rehydrate(ctx context.Context, srv provision.Server, imageID oci.ImageID) (*Rehydrated, error) {
	reg, err := registry.GetRegistry(ctx, srv.Env())
	if err != nil {
		return nil, err
	}

	var opts []name.Option
	if reg.IsInsecure() {
		opts = append(opts, name.Insecure)
	}

	ref, err := name.ParseReference(imageID.ImageRef(), opts...)
	if err != nil {
		return nil, err
	}

	img, err := remote.Image(ref, oci.RemoteOpts(ctx)...)
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
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return &r, nil
}

func MakeConfigTag(buildID provision.BuildID) provision.BuildID {
	return buildID.WithSuffix("config")
}
