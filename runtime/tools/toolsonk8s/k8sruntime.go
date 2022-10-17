// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package toolsonk8s

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing/module"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

type Runtime struct {
	Config cfg.Configuration
}

const toolNamespace = "fn-pipeline-tools"

func (k Runtime) CanConsumePublicImages() bool { return true }

func (k Runtime) RunWithOpts(ctx context.Context, opts rtypes.RunToolOpts, onStart func()) error {
	k8s, ck, err := k.makeRuntime(ctx)
	if err != nil {
		return err
	}

	if len(opts.Mounts) > 0 {
		return fnerrors.New("not supported: Mounts")
	}

	if opts.RunAsUser {
		return fnerrors.New("not supported: RunAsUser")
	}

	var imgid oci.ImageID
	if opts.PublicImageID == nil {
		if opts.ImageName == "" {
			return fnerrors.New("ImageName is required")
		}

		// XXX handle opts.NoNetworking
		imgid, err = tasks.Return(ctx, tasks.Action("kubernetes.invocation.push-image"), func(ctx context.Context) (oci.ImageID, error) {
			name, err := registry.RawAllocateName(ctx, ck, opts.ImageName)
			if err != nil {
				return oci.ImageID{}, err
			}

			resolvedName, err := compute.GetValue(ctx, name)
			if err != nil {
				return oci.ImageID{}, err
			}

			tasks.Attachments(ctx).AddResult("ref", resolvedName.ImageID.ImageRef())

			// XXX this ideally would have done by the parent, so we'd have parallelism.
			return oci.RawAsResolvable(opts.Image).Push(ctx, resolvedName, true)
		})
		if err != nil {
			return err
		}

	} else {
		imgid = *opts.PublicImageID
	}

	// XXX use more meaningful names.
	return k8s.RunAttachedOpts(ctx, toolNamespace, "tool-"+ids.NewRandomBase32ID(8), runtime.ContainerRunOpts{
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
	k8s, _, err := k.makeRuntime(ctx)
	if err != nil {
		return specs.Platform{}, err
	}

	platforms, err := k8s.UnmatchedTargetPlatforms(ctx)
	if err != nil {
		return specs.Platform{}, err
	}

	if len(platforms) == 0 {
		return specs.Platform{}, fnerrors.InternalError("no platform specified in kubernetes?")
	}

	return platforms[0], nil
}

func (kt Runtime) makeRuntime(ctx context.Context) (*kubernetes.Cluster, cfg.Configuration, error) {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return nil, nil, err
	}

	ck := cfg.MakeConfigurationWith("tools", kt.Config.Workspace(), cfg.ConfigurationSlice{
		Configuration:         root.DevHost().ConfigureTools,
		PlatformConfiguration: root.DevHost().ConfigurePlatform,
	})

	k, err := kubernetes.ConnectToCluster(ctx, ck)
	if err != nil {
		return nil, nil, err
	}

	return k, ck, nil
}