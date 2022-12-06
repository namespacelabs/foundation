// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package toolsonk8s

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/internal/runtime/rtypes"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

type Runtime struct {
	cluster       *kubernetes.Cluster
	configuration cfg.Configuration
}

const toolNamespace = "fn-pipeline-tools"

func (k Runtime) CanConsumePublicImages() bool { return true }

func (k Runtime) RunWithOpts(ctx context.Context, opts rtypes.RunToolOpts, onStart func()) error {
	if len(opts.Mounts) > 0 {
		return fnerrors.New("not supported: Mounts")
	}

	if opts.RunAsUser {
		return fnerrors.New("not supported: RunAsUser")
	}

	var imgid oci.ImageID
	if opts.ImageName == "" {
		return fnerrors.New("ImageName is required")
	}

	// XXX handle opts.NoNetworking
	var err error
	imgid, err = tasks.Return(ctx, tasks.Action("kubernetes.invocation.push-image"), func(ctx context.Context) (oci.ImageID, error) {
		reg, err := registry.GetRegistryFromConfig(ctx, "", k.configuration)
		if err != nil {
			return oci.ImageID{}, err
		}

		name := reg.AllocateName(opts.ImageName)

		resolvedName, err := compute.GetValue(ctx, name)
		if err != nil {
			return oci.ImageID{}, err
		}

		tasks.Attachments(ctx).AddResult("ref", resolvedName.ImageID.ImageRef())

		// XXX this ideally would have done by the parent, so we'd have parallelism.
		digest, err := oci.RawAsResolvable(opts.Image).Push(ctx, resolvedName.TargetRepository, true)
		if err != nil {
			return oci.ImageID{}, err
		}

		return resolvedName.WithDigest(digest), nil
	})
	if err != nil {
		return err
	}

	// XXX use more meaningful names.
	return k.cluster.RunAttachedOpts(ctx, toolNamespace, "tool-"+ids.NewRandomBase32ID(8), runtime.ContainerRunOpts{
		Image:      imgid,
		WorkingDir: opts.WorkingDir,
		Command:    opts.Command,
		Args:       opts.Args,
		Env:        opts.Env,
	}, runtime.TerminalIO{
		TTY:    opts.AllocateTTY,
		Stdin:  opts.Stdin,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
	}, onStart)
}

func (k Runtime) HostPlatform(ctx context.Context) (specs.Platform, error) {
	platforms, err := k.cluster.UnmatchedTargetPlatforms(ctx)
	if err != nil {
		return specs.Platform{}, err
	}

	if len(platforms) == 0 {
		return specs.Platform{}, fnerrors.InternalError("no platform specified in kubernetes?")
	}

	return platforms[0], nil
}

func MakeRuntime(ctx context.Context) (Runtime, error) {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return Runtime{}, err
	}

	ck := cfg.MakeConfigurationWith("tools", root.Workspace(), cfg.ConfigurationSlice{
		Configuration:         root.DevHost().ConfigureTools,
		PlatformConfiguration: root.DevHost().ConfigurePlatform,
	})

	k, err := kubernetes.ConnectToCluster(ctx, ck)

	return Runtime{k, ck}, err
}
