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
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
)

const stackBinaryPb = "config/stack.binarypb"

func Rehydrate(ctx context.Context, srv provision.Server, imageID oci.ImageID) (*schema.Stack, error) {
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

	contents, err := oci.ReadFileFromImage(ctx, img, stackBinaryPb)
	if err != nil {
		return nil, err
	}

	stack := &schema.Stack{}
	if err := proto.Unmarshal(contents, stack); err != nil {
		return nil, err
	}

	return stack, nil
}

func MakeConfigTag(buildID provision.BuildID) provision.BuildID {
	return buildID.WithSuffix("config")
}