// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tools

import (
	"context"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/module"
	"namespacelabs.dev/go-ids"
)

type k8stools struct{}

const toolNamespace = "fn-pipelines"

func (k k8stools) RunWithOpts(ctx context.Context, opts rtypes.RunToolOpts, onStart func()) error {
	k8s, ck, err := k.k8s(ctx)
	if err != nil {
		return err
	}

	if len(opts.Mounts) > 0 {
		return fnerrors.New("not supported: Mounts")
	}

	if opts.RunAsUser {
		return fnerrors.New("not supported: RunAsUser")
	}

	if opts.ImageName == "" {
		return fnerrors.New("ImageName is required")
	}

	// XXX handle opts.NoNetworking

	name, err := registry.RawAllocateName(ctx, ck, opts.ImageName)
	if err != nil {
		return err
	}

	resolvedName, err := compute.GetValue(ctx, name)
	if err != nil {
		return err
	}

	// XXX this ideally would have done by the parent, so we'd have parallelism.
	imgid, err := oci.RawAsResolvable(opts.Image).Push(ctx, resolvedName)
	if err != nil {
		return err
	}

	// XXX use more meaningful names.
	return k8s.RunAttachedOpts(ctx, toolNamespace, "tool-"+ids.NewRandomBase32ID(8), runtime.ServerRunOpts{
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

func (k k8stools) HostPlatform(ctx context.Context) (specs.Platform, error) {
	k8s, _, err := k.k8s(ctx)
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

func (k8stools) k8s(ctx context.Context) (kubernetes.Unbound, *devhost.ConfigKey, error) {
	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return kubernetes.Unbound{}, nil, err
	}

	ck := &devhost.ConfigKey{DevHost: root.DevHost, Selector: devhost.ForToolsRuntime()}

	k, err := kubernetes.New(ctx, ck.DevHost, ck.Selector)
	if err != nil {
		return kubernetes.Unbound{}, nil, err
	}

	return k, ck, nil
}
