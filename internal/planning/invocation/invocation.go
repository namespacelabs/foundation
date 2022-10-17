// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package invocation

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/build"
	"namespacelabs.dev/foundation/build/assets"
	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type Invocation struct {
	ImageName            string
	Image                compute.Computable[oci.ResolvableImage]
	PublicImageID        *oci.ImageID
	SupportedToolVersion int
	Command              []string
	Args                 []string
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
	p, err := tools.HostPlatform(ctx, env.Configuration())
	if err != nil {
		return nil, err
	}

	return MakeForPlatforms(ctx, pl, env, serverLocRef, with, p)
}

func MakeForPlatforms(ctx context.Context, pl pkggraph.SealedPackageLoader, env cfg.Context, serverLocRef *pkggraph.Location, with *schema.Invocation, target ...specs.Platform) (*Invocation, error) {
	var binRef *schema.PackageRef
	if with.BinaryRef != nil {
		binRef = with.BinaryRef
	} else if with.Binary != "" {
		binRef = schema.MakePackageSingleRef(schema.MakePackageName(with.Binary))
	} else {
		return nil, fnerrors.UserError(nil, "`binary` is required to point to a binary package")
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

	invocation := &Invocation{
		ImageName:  bin.Name,
		Command:    prepared.Command,
		Image:      image,
		WorkingDir: with.WorkingDir,
		NoCache:    with.NoCache,
		Inject:     with.Inject,
	}

	if prebuilt, is := build.IsPrebuilt(prepared.Plan.Spec); is {
		// The assumption at the moment is that all prebuilts are public.
		invocation.PublicImageID = &prebuilt
	}

	if v := pkg.Location.Module.Workspace.GetFoundation().GetToolsVersion(); v != 0 {
		invocation.SupportedToolVersion = int(v)
	}

	invocation.Args = with.Args

	for k, v := range with.Snapshots {
		if serverLocRef == nil {
			return nil, fnerrors.UserError(nil, "snapshots are not allowed in this context")
		}

		serverLoc := *serverLocRef

		if v.FromWorkspace == "" {
			return nil, fnerrors.UserError(serverLoc, "fromSnapshot can't be empty")
		}

		st, err := os.Stat(filepath.Join(serverLoc.Module.Abs(), v.FromWorkspace))
		if os.IsNotExist(err) {
			if v.Optional {
				continue
			}

			return nil, fnerrors.UserError(serverLoc, "required location %q does not exist", v.FromWorkspace)
		}

		var fsys fs.FS
		if st.IsDir() {
			if v.RequireFile {
				return nil, fnerrors.UserError(serverLoc, "%s: must be a file, not a directory", v.FromWorkspace)
			}

			v, err := compute.GetValue(ctx, serverLoc.Module.VersionedFS(v.FromWorkspace, false))
			if err != nil {
				return nil, fnerrors.UserError(serverLoc, "failed to read contents: %v", err)
			}

			fsys = v.FS()
		} else {
			contents, err := fs.ReadFile(serverLoc.Module.ReadWriteFS(), v.FromWorkspace)
			if err != nil {
				return nil, fnerrors.UserError(serverLoc, "failed to read file contents: %v", err)
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
			return nil, fnerrors.UserError(nil, "requiresKeys is not allowed in this context")
		}

		keySnapshot, err := keys.Collect(ctx)
		if err != nil {
			return nil, fnerrors.Wrapf(*serverLocRef, err, "setting up keys")
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
