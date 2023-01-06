// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gcloud

import (
	"context"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/schema"
)

func Run(ctx context.Context, io rtypes.IO, args []string) error {
	dir, err := os.UserHomeDir()
	if err != nil {
		return fnerrors.InternalError("failed to determine home")
	}

	gcloudDir := filepath.Join(dir, ".config/gcloud")
	if err := os.MkdirAll(gcloudDir, 0755); err != nil {
		return fnerrors.InternalError("failed to create %q: %w", gcloudDir, err)
	}

	hostPlatform := docker.HostPlatform()

	const imageName = "gcr.io/google.com/cloudsdktool/google-cloud-cli:412.0.0"
	imageRef := oci.ImageP(imageName, &hostPlatform, oci.ResolveOpts{PublicImage: true})
	image, err := compute.GetValue(ctx, imageRef)
	if err != nil {
		return err
	}

	var opts rtypes.RunToolOpts
	opts.Image = image
	opts.ImageName = imageName
	opts.IO = io
	opts.Command = []string{"gcloud"}
	opts.Args = args
	opts.RunAsUser = true
	opts.WorkingDir = "/"
	opts.Env = append(opts.Env, &schema.BinaryConfig_EnvEntry{
		Name:  "CLOUDSDK_CONFIG",
		Value: "/gcloudconfig",
	})
	opts.Mounts = append(opts.Mounts, &rtypes.LocalMapping{
		HostPath:      gcloudDir,
		ContainerPath: "/gcloudconfig",
	})

	return docker.Runtime().Run(ctx, opts)
}
