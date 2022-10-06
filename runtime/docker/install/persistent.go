// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package install

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/go-containerregistry/pkg/name"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/runtime/docker"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type PersistentSpec struct {
	Name              string
	ContainerName     string
	Version           string
	Image             string
	WaitUntilRunning  func(context.Context, string) error
	Volumes           map[string]string
	Ports             map[int]int
	Privileged        bool
	UseHostNetworking bool
}

type PersistentInformation struct {
	Installed       bool
	Running         bool
	Version         string
	HaveHostNetwork bool
}

func (p PersistentSpec) Ensure(ctx context.Context, progress io.Writer) error {
	return tasks.Action(p.Name+".check").Arg("version", p.Version).Run(ctx, func(ctx context.Context) error {
		cli, err := docker.NewClient()
		if err != nil {
			return err
		}
		defer cli.Close()

		info, err := p.running(ctx, cli)
		if err != nil {
			return err
		}

		if info.Version != p.Version || info.HaveHostNetwork != p.UseHostNetworking {
			if err := p.remove(ctx, cli); err != nil {
				return err
			}

			if err := p.install(ctx, cli, progress); err != nil {
				return err
			}
		}

		if !info.Running {
			return p.start(ctx, cli)
		}

		return nil
	})
}

func (p PersistentSpec) start(ctx context.Context, cli docker.Client) error {
	fmt.Fprintf(console.Debug(ctx), "Starting %s version %s\n", p.Name, p.Version)

	if err := cli.ContainerStart(ctx, p.ContainerName, types.ContainerStartOptions{}); err != nil {
		return err
	}

	if p.WaitUntilRunning == nil {
		return nil
	}

	return p.WaitUntilRunning(ctx, p.ContainerName)
}

func (p PersistentSpec) install(ctx context.Context, cli docker.Client, progress io.Writer) error {
	var imageID oci.ImageID
	imageID.Repository = p.Image
	imageID.Tag = p.Version

	image, err := compute.GetValue(ctx, oci.ResolveImage(imageID.ImageRef(), docker.HostPlatform()).Image())
	if err != nil {
		return err
	}

	tag, err := name.NewTag(imageID.ImageRef())
	if err != nil {
		return err
	}

	if err := docker.WriteImage(ctx, image, tag, true); err != nil {
		return err
	}

	config := &container.Config{
		Image:   imageID.ImageRef(),
		Volumes: map[string]struct{}{},
	}

	host := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "always"},
		Privileged:    p.Privileged,
	}

	host.PortBindings = map[nat.Port][]nat.PortBinding{}

	for hostPort, containerPort := range p.Ports {
		parsed, err := nat.ParsePortSpec(fmt.Sprintf("127.0.0.1:%d:%d", hostPort, containerPort))
		if err != nil {
			return err
		}

		for _, p := range parsed {
			host.PortBindings[p.Port] = append(host.PortBindings[p.Port], p.Binding)
		}
	}

	for name, target := range p.Volumes {
		host.Binds = append(host.Binds, fmt.Sprintf("%s:%s", name, target))
	}

	if p.UseHostNetworking {
		host.NetworkMode = container.NetworkMode("host")
	}

	created, err := tasks.Return(ctx, tasks.Action("docker.container.create").Arg("name", p.ContainerName), func(ctx context.Context) (container.ContainerCreateCreatedBody, error) {
		return cli.ContainerCreate(ctx, config, host, &network.NetworkingConfig{}, nil, p.ContainerName)
	})
	if err != nil {
		return err
	}

	if err := tasks.Action("docker.container.start").Arg("name", p.ContainerName).Arg("id", created.ID).Run(ctx, func(ctx context.Context) error {
		return cli.ContainerStart(ctx, created.ID, types.ContainerStartOptions{})
	}); err != nil {
		return err
	}

	if p.WaitUntilRunning == nil {
		return nil
	}

	return p.WaitUntilRunning(ctx, p.ContainerName)
}

func (p PersistentSpec) remove(ctx context.Context, cli docker.Client) error {
	err := cli.ContainerRemove(ctx, p.ContainerName, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true})
	if client.IsErrNotFound(err) {
		return nil
	}
	return err
}

func (p PersistentSpec) running(ctx context.Context, cli docker.Client) (*PersistentInformation, error) {
	res, err := cli.ContainerInspect(ctx, p.ContainerName)
	if err != nil {
		if client.IsErrNotFound(err) {
			return &PersistentInformation{Installed: false}, nil
		}

		return nil, err
	}

	var info PersistentInformation

	if _, ok := res.NetworkSettings.Networks["host"]; ok {
		info.HaveHostNetwork = true
	}

	info.Installed = true
	info.Running = res.State.Running

	n, err := name.ParseReference(res.Config.Image)
	if err != nil {
		return nil, err
	}

	if tag, ok := n.(name.Tag); ok {
		info.Version = tag.TagStr()
	}

	return &info, nil
}
