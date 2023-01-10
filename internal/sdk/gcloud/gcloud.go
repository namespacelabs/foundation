// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/natefinch/atomic"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/common"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

func IsOnCloudBuild() bool {
	return os.Getenv("NS_CLOUD_BUILD") != ""
}

func Run(ctx context.Context, io rtypes.IO, args ...string) error {
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
	imageRef := oci.ImageP(imageName, &hostPlatform, oci.RegistryAccess{PublicImage: true})
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
	if IsOnCloudBuild() {
		opts.Network = "cloudbuild"
	} else {
		opts.Env = append(opts.Env, &schema.BinaryConfig_EnvEntry{
			Name:  "CLOUDSDK_CONFIG",
			Value: "/gcloudconfig",
		})
		opts.Mounts = append(opts.Mounts, &rtypes.LocalMapping{
			HostPath:      gcloudDir,
			ContainerPath: "/gcloudconfig",
		})
	}

	return docker.Runtime().Run(ctx, opts)
}

type credential struct {
	AccessToken string    `json:"access_token"`
	IDToken     string    `json:"id_token"`
	TokenExpiry time.Time `json:"token_expiry"`
}

type ConfigHelper struct {
	Credential credential `json:"credential"`
}

func Credentials(ctx context.Context) (*credential, error) {
	return tasks.Return(ctx, tasks.Action("gcloud.fetch-access-token"), func(ctx context.Context) (*credential, error) {
		cacheDir, err := dirs.Ensure(cacheDir())
		if err != nil {
			return nil, err
		}

		cacheFile := filepath.Join(cacheDir, "access_token.json")
		contents, err := os.ReadFile(cacheFile)
		if err == nil {
			var cred credential
			if json.Unmarshal(contents, &cred) == nil {
				if !expired(&cred) {
					return &cred, nil
				}
			}
		}

		h, err := Helper(ctx)
		if err != nil {
			return nil, err
		}

		var b bytes.Buffer
		if json.NewEncoder(&b).Encode(h.Credential) == nil {
			_ = atomic.WriteFile(cacheFile, bytes.NewReader(b.Bytes()))
		}

		return &h.Credential, nil
	})
}

func cacheDir() (string, error) {
	c, err := dirs.Config()
	if err == nil {
		return filepath.Join(c, "gcloud-credcache"), nil
	}
	return c, err
}

func Helper(ctx context.Context) (*ConfigHelper, error) {
	return tasks.Return(ctx, tasks.Action("gcloud.slow-fetch-access-token"), func(ctx context.Context) (*ConfigHelper, error) {
		var out bytes.Buffer
		stderr := console.TypedOutput(ctx, "gcloud", common.CatOutputTool)

		if err := Run(ctx, rtypes.IO{Stdout: &out, Stderr: stderr}, "config", "config-helper", "--format=json"); err != nil {
			return nil, err
		}

		var h ConfigHelper
		if err := json.Unmarshal(out.Bytes(), &h); err != nil {
			return nil, fnerrors.InternalError("failed to decode gcloud output: %w", err)
		}

		return &h, nil
	})
}
