// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package unpack

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func WriteLocal(absPath string, perm os.FileMode, ref artifacts.Reference) compute.Computable[string] {
	return &writeLocal{absPath: absPath, perm: perm, download: download.URL(ref), url: ref.URL}
}

type writeLocal struct {
	url      string
	absPath  string
	perm     os.FileMode
	download compute.Computable[bytestream.ByteStream]

	compute.LocalScoped[string]
}

func (wl *writeLocal) Action() *tasks.ActionEvent {
	return tasks.Action("artifact.unpack").Arg("absPath", wl.absPath).Arg("perm", wl.perm).Arg("url", wl.url)
}

func (wl *writeLocal) Inputs() *compute.In {
	return compute.Inputs().Indigestible("absPath", wl.absPath).Computable("download", wl.download)
}

func (wl *writeLocal) Output() compute.Output {
	return compute.Output{NotCacheable: true}
}

func (wl *writeLocal) Compute(ctx context.Context, deps compute.Resolved) (string, error) {
	if err := os.MkdirAll(filepath.Dir(wl.absPath), 0755); err != nil {
		return "", err
	}

	download := compute.GetDepValue(deps, wl.download, "download")

	dir := filepath.Dir(wl.absPath)
	name := filepath.Base(wl.absPath)

	digest, err := bytestream.Digest(ctx, download)
	if err != nil {
		return "", err
	}

	if err := fnfs.WriteFileExtended(ctx, fnfs.ReadWriteLocalFS(dir), name, wl.perm,
		fnfs.WriteFileExtendedOpts{
			ContentsDigest: digest,
			EnsureFileMode: true,
		},
		func(w io.Writer) error {
			r, err := download.Reader()
			if err != nil {
				return err
			}
			defer r.Close()

			p := artifacts.NewProgressWriter(0, nil)
			tasks.Attachments(ctx).SetProgress(p)

			if _, err := io.Copy(io.MultiWriter(w, p), r); err != nil {
				return err
			}

			return nil
		}); err != nil {
		return "", err
	}

	return wl.absPath, nil
}
