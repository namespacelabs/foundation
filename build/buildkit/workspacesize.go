// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/dustin/go-humanize"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
)

func reportWorkspaceSizeErr(ctx context.Context, fsys fs.FS, totalSize uint64) error {
	type fileAndSize struct {
		Name string
		Size uint64
	}

	var fileList []fileAndSize

	var description string
	if err := fnfs.VisitFiles(ctx, fsys, func(path string, _ bytestream.ByteStream, de fs.DirEntry) error {
		if !de.IsDir() {
			fi, err := de.Info()
			if err == nil {
				fileList = append(fileList, fileAndSize{path, uint64(fi.Size())})
			}
		}
		return nil
	}); err == nil {
		slices.SortFunc(fileList, func(a, b fileAndSize) bool {
			return a.Size > b.Size
		})

		if len(fileList) > 10 {
			fileList = fileList[:10]
		}

		fileLabel := make([]string, len(fileList))
		for k, l := range fileList {
			fileLabel[k] = fmt.Sprintf("    %s (%s)", l.Name, humanize.Bytes(l.Size))
		}

		description = fmt.Sprintf("  The top %d largest files in the workspace are:\n\n%s", len(fileLabel), strings.Join(fileLabel, "\n"))
	} else {
		description = "Wasn't able to compute the largest files in the workspace."
	}

	return fnerrors.New(`the workspace snapshot is unexpectedly large (%s vs max expected %s);
this is likely a problem with the way that foundation is filtering workspace contents.

%s

If you don't think this is an actual issue, please re-run with --skip_buildkit_workspace_size_check=true.`,
		humanize.Bytes(totalSize), humanize.Bytes(maxExpectedWorkspaceSize), description)
}
