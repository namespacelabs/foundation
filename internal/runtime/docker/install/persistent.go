// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package install

import (
	"context"
	"fmt"
	"io"
	"net/netip"
	"strconv"

	"github.com/containerd/errdefs"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/parsing/platform"
	"namespacelabs.dev/foundation/internal/runtime/docker"
	"namespacelabs.dev/foundation/std/tasks"
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

	if err := cli.ContainerStart(ctx, p.ContainerName, client.ContainerStartOptions{}); err != nil {
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

	hostPlatform := platform.RuntimePlatform()
	hostPlatform.OS = "linux"

	image, err := compute.GetValue(ctx, oci.ResolveImage(imageID.ImageRef(), hostPlatform).Image())
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

	host.PortBindings = network.PortMap{}

	for hostPort, containerPort := range p.Ports {
		port, ok := network.PortFrom(uint16(containerPort), network.TCP)
		if !ok {
			return fmt.Errorf("invalid container port %d", containerPort)
		}

		host.PortBindings[port] = append(host.PortBindings[port], network.PortBinding{
			HostIP:   netip.MustParseAddr("127.0.0.1"),
			HostPort: strconv.Itoa(hostPort),
		})
	}

	for name, target := range p.Volumes {
		host.Binds = append(host.Binds, fmt.Sprintf("%s:%s", name, target))
	}

	if p.UseHostNetworking {
		host.NetworkMode = container.NetworkMode("host")
	}

	created, err := tasks.Return(ctx, tasks.Action("docker.container.create").Arg("name", p.ContainerName), func(ctx context.Context) (client.ContainerCreateResult, error) {
		return cli.ContainerCreate(ctx, config, host, &network.NetworkingConfig{}, nil, p.ContainerName)
	})
	if err != nil {
		return err
	}

	if err := tasks.Action("docker.container.start").Arg("name", p.ContainerName).Arg("id", created.ID).Run(ctx, func(ctx context.Context) error {
		return cli.ContainerStart(ctx, created.ID, client.ContainerStartOptions{})
	}); err != nil {
		return err
	}

	if p.WaitUntilRunning == nil {
		return nil
	}

	return p.WaitUntilRunning(ctx, p.ContainerName)
}

func (p PersistentSpec) remove(ctx context.Context, cli docker.Client) error {
	err := cli.ContainerRemove(ctx, p.ContainerName, client.ContainerRemoveOptions{RemoveVolumes: true, Force: true})
	if errdefs.IsNotFound(err) {
		return nil
	}
	return err
}

func (p PersistentSpec) running(ctx context.Context, cli docker.Client) (*PersistentInformation, error) {
	res, err := cli.ContainerInspect(ctx, p.ContainerName)
	if err != nil {
		if errdefs.IsNotFound(err) {
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
