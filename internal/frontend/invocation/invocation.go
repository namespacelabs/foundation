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

	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/compute"
)

type Invocation struct {
	ImageName  string
	Image      compute.Computable[oci.Image]
	Command    []string
	Args       []string
	Snapshots  []Snapshot
	WorkingDir string
	NoCache    bool
	Inject     []*schema.Invocation_ValueInjection
}

type Snapshot struct {
	Name     string
	Contents fs.FS
}

func Make(ctx context.Context, env provision.ServerEnv, serverLocRef *workspace.Location, with *schema.Invocation) (*Invocation, error) {
	if with.Binary == "" {
		return nil, fnerrors.UserError(nil, "`binary` is required to point to a binary package")
	}

	binPkg, err := env.LoadByName(ctx, schema.PackageName(with.Binary))
	if err != nil {
		return nil, err
	}

	p, err := tools.HostPlatform(ctx)
	if err != nil {
		return nil, err
	}

	bin, err := binary.PlanImage(ctx, binPkg, env, true, &p)
	if err != nil {
		return nil, err
	}

	invocation := &Invocation{
		ImageName:  bin.Name,
		Command:    bin.Command,
		Image:      bin.Image,
		WorkingDir: with.WorkingDir,
		NoCache:    with.NoCache,
		Inject:     with.Inject,
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

			fsys, err = serverLoc.Module.SnapshotContents(ctx, v.FromWorkspace)
			if err != nil {
				return nil, fnerrors.UserError(serverLoc, "failed to read contents: %v", err)
			}
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

	sort.Strings(invocation.Args)

	sort.Slice(invocation.Snapshots, func(i, j int) bool {
		return strings.Compare(invocation.Snapshots[i].Name, invocation.Snapshots[j].Name) < 0
	})

	return invocation, nil
}
