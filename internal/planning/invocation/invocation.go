// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package invocation

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/build/assets"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/support"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type Invocation struct {
	Buildkit             buildkit.ClientFactory
	ImageName            string
	Image                compute.Computable[oci.ResolvableImage]
	SupportedToolVersion int
	Command              []string
	Args                 []string
	Env                  []*schema.BinaryConfig_EnvEntry
	Snapshots            []Snapshot
	WorkingDir           string
	NoCache              bool
	Inject               []*schema.Invocation_ValueInjection
}

type Snapshot struct {
	Name     string
	Contents fs.FS
}

func Make(ctx context.Context, pl pkggraph.SealedPackageLoader, env cfg.Context, serverLocRef *pkggraph.Location, with *schema.Invocation) (*Invocation, error) {
	cli, err := buildkit.Client(ctx, env.Configuration(), nil)
	if err != nil {
		return nil, err
	}

	return makeForPlatforms(ctx, cli, pl, env, serverLocRef, with, cli.BuildkitOpts().HostPlatform)
}

func makeForPlatforms(ctx context.Context, cli *buildkit.GatewayClient, pl pkggraph.SealedPackageLoader, env cfg.Context, serverLocRef *pkggraph.Location, with *schema.Invocation, target ...specs.Platform) (*Invocation, error) {
	var binRef *schema.PackageRef
	if with.BinaryRef != nil {
		binRef = with.BinaryRef
	} else if with.Binary != "" {
		binRef = schema.MakePackageSingleRef(schema.MakePackageName(with.Binary))
	} else {
		return nil, fnerrors.New("`binary` is required to point to a binary package")
	}

	pkg, bin, err := pkggraph.LoadBinary(ctx, pl, binRef)
	if err != nil {
		return nil, err
	}

	prepared, err := binary.PlanBinary(ctx, pl, env, pkg.Location, bin, assets.AvailableBuildAssets{}, binary.BuildImageOpts{
		UsePrebuilts: true,
		Platforms:    target,
	})
	if err != nil {
		return nil, err
	}

	image, err := prepared.Image(ctx, pkggraph.MakeSealedContext(env, pl))
	if err != nil {
		return nil, err
	}

	workingDir := with.WorkingDir
	if prepared.WorkingDir != "" {
		workingDir = prepared.WorkingDir
	}

	invocation := &Invocation{
		Buildkit:   cli,
		ImageName:  bin.Name,
		Command:    prepared.Command,
		Args:       prepared.Args,
		Env:        prepared.Env,
		WorkingDir: workingDir,
		Image:      image,
		NoCache:    with.NoCache,
		Inject:     with.Inject,
	}

	if v := pkg.Location.Module.Workspace.GetFoundation().GetToolsVersion(); v != 0 {
		invocation.SupportedToolVersion = int(v)
	}

	invocation.Args = append(invocation.Args, with.Args...)
	invocation.Env, err = support.MergeEnvs(invocation.Env, with.Env)
	if err != nil {
		return nil, err
	}

	for k, v := range with.Snapshots {
		if serverLocRef == nil {
			return nil, fnerrors.New("snapshots are not allowed in this context")
		}

		serverLoc := *serverLocRef

		if v.FromWorkspace == "" {
			return nil, fnerrors.NewWithLocation(serverLoc, "fromSnapshot can't be empty")
		}

		st, err := os.Stat(filepath.Join(serverLoc.Module.Abs(), v.FromWorkspace))
		if os.IsNotExist(err) {
			if v.Optional {
				continue
			}

			return nil, fnerrors.NewWithLocation(serverLoc, "required location %q does not exist", v.FromWorkspace)
		}

		var fsys fs.FS
		if st.IsDir() {
			if v.RequireFile {
				return nil, fnerrors.NewWithLocation(serverLoc, "%s: must be a file, not a directory", v.FromWorkspace)
			}

			v, err := compute.GetValue(ctx, serverLoc.Module.VersionedFS(v.FromWorkspace, false))
			if err != nil {
				return nil, fnerrors.NewWithLocation(serverLoc, "failed to read contents: %v", err)
			}

			fsys = v.FS()
		} else {
			contents, err := fs.ReadFile(serverLoc.Module.ReadWriteFS(), v.FromWorkspace)
			if err != nil {
				return nil, fnerrors.NewWithLocation(serverLoc, "failed to read file contents: %v", err)
			}

			m := &memfs.FS{}
			m.Add(st.Name(), contents)
			fsys = m
		}

		invocation.Snapshots = append(invocation.Snapshots, Snapshot{
			Name:     k,
			Contents: fsys,
		})
	}

	// XXX security validate this; ideally the tool would RPC back to `ns` to have something decrypted.
	// We're ok doing this for now because tools' runtime invocation is hermetic.
	if with.RequiresKeys {
		if serverLocRef == nil {
			return nil, fnerrors.New("requiresKeys is not allowed in this context")
		}

		keySnapshot, err := keys.Collect(ctx)
		if err != nil {
			return nil, fnerrors.NewWithLocation(*serverLocRef, "setting up keys failed: %w", err)
		}

		invocation.Snapshots = append(invocation.Snapshots, Snapshot{
			Name:     keys.SnapshotKeys,
			Contents: keySnapshot,
		})
	}

	// Make configuration deterministic.
	sort.Slice(invocation.Snapshots, func(i, j int) bool {
		return strings.Compare(invocation.Snapshots[i].Name, invocation.Snapshots[j].Name) < 0
	})

	return invocation, nil
}
