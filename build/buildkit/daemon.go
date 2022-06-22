// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"fmt"

	"github.com/cenkalti/backoff/v4"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	buildkit "github.com/moby/buildkit/client"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/runtime/docker/install"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const DefaultContainerName = "fn-buildkitd"

func EnsureBuildkitd(ctx context.Context, containerName string) (*Instance, error) {
	vendoredVersion, err := Version()
	if err != nil {
		return nil, err
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

func RemoveBuildkitd(ctx context.Context) error {
	dockerclient, err := docker.NewClient()
	if err != nil {
		return fnerrors.InternalError("failed to instantiate the docker client while removing buildkitd: %w", err)
	}
	// Ignore if the container is already removed.
	ctr, err := dockerclient.ContainerInspect(ctx, DefaultContainerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return nil
		} else {
			return err
		}
	}
	opts := types.ContainerRemoveOptions{Force: true}
	if err := dockerclient.ContainerRemove(ctx, ctr.Name, opts); err != nil {
		return fnerrors.InternalError("failed to remove the buildkitd container: %w", err)
	}
	return nil
}

func makeAddr(containerName string) string {
	return fmt.Sprintf("docker-container://%s", containerName)
}

func waitForBuildkit(ctx context.Context, containerName string) error {
	return tasks.Action("buildkit.wait-until-ready").Run(ctx, func(ctx context.Context) error {
		return backoff.Retry(func() error {
			c, err := buildkit.New(ctx, makeAddr(containerName))
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
