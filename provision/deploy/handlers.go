// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"namespacelabs.dev/foundation/build/binary"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/provision/tool"
	"namespacelabs.dev/foundation/runtime/rtypes"
	"namespacelabs.dev/foundation/runtime/tools"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func computeHandlers(ctx context.Context, in *stack.Stack) ([]*tool.Definition, error) {
	var handlers []*tool.Definition
	for k, s := range in.ParsedServers {
		for _, n := range s.Deps {
			h, err := parseHandlers(ctx, in.Servers[k], n)
			if err != nil {
				return nil, err
			}
			handlers = append(handlers, h...)
		}
	}

	sort.SliceStable(handlers, func(i, j int) bool {
		if handlers[i].For == handlers[j].For {
			return strings.Compare(handlers[i].Source.PackageName.String(), handlers[j].Source.PackageName.String()) < 0
		}

		return strings.Compare(handlers[i].For.String(), handlers[j].For.String()) < 0
	})

	return handlers, nil
}

func parseHandlers(ctx context.Context, server provision.Server, pr *stack.ParsedNode) ([]*tool.Definition, error) {
	pkg := pr.Package
	source := tool.Source{
		PackageName: pkg.PackageName(),
		// The server in context is always implicitly declared.
		DeclaredStack: append([]schema.PackageName{server.PackageName()}, pr.ProvisionPlan.DeclaredStack...),
	}

	// Determinism.
	sort.Slice(source.DeclaredStack, func(i, j int) bool {
		return strings.Compare(source.DeclaredStack[i].String(), source.DeclaredStack[j].String()) < 0
	})

	var handlers []*tool.Definition
	if dec := pr.ProvisionPlan.Provisioning; dec != nil {
		if dec.Binary == "" {
			return nil, fnerrors.UserError(pkg.Location, "configure.with.binary is required to point to a binary package")
		}

		bin, err := server.Env().LoadByName(ctx, schema.PackageName(dec.Binary))
		if err != nil {
			return nil, err
		}

		p := tools.Impl().HostPlatform()
		prepared, err := binary.PlanImage(ctx, bin, server.Env(), true, &p)
		if err != nil {
			return nil, err
		}

		invocation, err := makeInvocation(ctx, server.Env(), server.Location, *dec, prepared)
		if err != nil {
			return nil, err
		}

		handlers = append(handlers, &tool.Definition{
			For:           server.PackageName(),
			ServerAbsPath: server.Location.Abs(),
			Source:        source,
			Invocation:    invocation,
		})
	}

	return handlers, nil
}

func makeInvocation(ctx context.Context, env ops.Environment, serverLoc workspace.Location, with frontend.Invocation, bin *binary.PreparedImage) (*tool.Invocation, error) {
	invocation := &tool.Invocation{
		ImageName:  bin.Name,
		Command:    bin.Command,
		Image:      bin.Image,
		WorkingDir: with.WorkingDir,
		NoCache:    with.NoCache,
	}

	for target, mount := range with.Mounts {
		// XXX verify we're not breaking from the workspace.
		invocation.Mounts = append(invocation.Mounts, &rtypes.LocalMapping{
			LocalPath:     mount.FromWorkspace,
			ContainerPath: target,
		})
	}

	invocation.Args = with.Args

	for k, v := range with.Snapshots {
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

		invocation.Snapshots = append(invocation.Snapshots, tool.Snapshot{
			Name:     k,
			Contents: fsys,
		})
	}

	// XXX security validate this; ideally the tool would RPC back to `fn` to have something decrypted.
	// We're ok doing this for now because tools' runtime invocation is hermetic.
	if with.RequiresKeys {
		keySnapshot, err := keys.Collect(ctx)
		if err != nil {
			return nil, fnerrors.Wrapf(serverLoc, err, "setting up keys")
		}

		invocation.Snapshots = append(invocation.Snapshots, tool.Snapshot{
			Name:     keys.SnapshotKeys,
			Contents: keySnapshot,
		})
	}

	// Make configuration deterministic.

	sort.Slice(invocation.Mounts, func(i, j int) bool {
		i_l, i_j := invocation.Mounts[i].LocalPath, invocation.Mounts[j].LocalPath
		if i_l == i_j {
			return strings.Compare(invocation.Mounts[i].ContainerPath, invocation.Mounts[j].ContainerPath) < 0
		}

		return strings.Compare(i_l, i_j) < 0
	})

	sort.Strings(invocation.Args)

	sort.Slice(invocation.Snapshots, func(i, j int) bool {
		return strings.Compare(invocation.Snapshots[i].Name, invocation.Snapshots[j].Name) < 0
	})

	return invocation, nil
}
