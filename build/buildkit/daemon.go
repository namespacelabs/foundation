// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/cenkalti/backoff/v4"
	buildkit "github.com/moby/buildkit/client"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/docker/install"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const DefaultContainerName = "fn-buildkitd"

func EnsureBuildkitd(ctx context.Context, containerName string) (*Instance, error) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return nil, fnerrors.InternalError("no builtin debug information?")
	}

	var vendoredVersion string
	for _, d := range bi.Deps {
		if d.Path == "github.com/moby/buildkit" {
			vendoredVersion = d.Version
			break
		}
	}

	if vendoredVersion == "" {
		return nil, fnerrors.InternalError("buildkit: vendored version is empty")
	}

	var spec = install.PersistentSpec{
		Name:             "buildkit",
		ContainerName:    containerName,
		Version:          vendoredVersion,
		Image:            "moby/buildkit",
		WaitUntilRunning: waitForBuildkit,
		Volumes: map[string]string{
			containerName: "/var/lib/buildkit",
		},
		Privileged:        true,
		UseHostNetworking: true, // we need to be able to access APIs that are hosted by the host.
	}

	if err := spec.Ensure(ctx, console.TypedOutput(ctx, "docker", console.CatOutputTool)); err != nil {
		return nil, err
	}

	return &Instance{
		Addr: makeAddr(spec.ContainerName),
	}, nil
}

func makeAddr(containerName string) string {
	return fmt.Sprintf("docker-container://%s", containerName)
}

func waitForBuildkit(ctx context.Context, containerName string) error {
	return tasks.Action("buildkit.wait-until-ready").Run(ctx, func(ctx context.Context) error {
		return backoff.Retry(func() error {
			c, err := buildkit.New(ctx, "docker-container://"+containerName)
			if err != nil {
				return err
			}

			defer c.Close()

			_, err = c.ListWorkers(ctx)
			return err
		}, backoff.WithContext(backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 10), ctx))
	})
}

func setupBuildkit(ctx context.Context, conf *Overrides) (*Instance, error) {
	if conf.BuildkitAddr != "" {
		// XXX no version check is performed.
		return &Instance{
			Addr: conf.BuildkitAddr,
		}, nil
	}

	return EnsureBuildkitd(ctx, conf.ContainerName)
}
