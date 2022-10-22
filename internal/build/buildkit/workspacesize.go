// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"fmt"
	"io/fs"
	"strings"

	"github.com/dustin/go-humanize"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
)

type fileAndSize struct {
	Name string
	Size uint64
}

type workspaceSizeReport struct {
	Files     []fileAndSize
	TotalSize uint64
}

func reportWorkspaceSize(ctx context.Context, fsys fs.FS, matcher *fnfs.PatternMatcher) (workspaceSizeReport, error) {
	var w workspaceSizeReport

	err := fnfs.WalkDirWithMatcher(fsys, ".", matcher, func(path string, d fs.DirEntry) error {
		if !d.IsDir() {
			fi, err := d.Info()
			if err == nil {
				w.Files = append(w.Files, fileAndSize{path, uint64(fi.Size())})
				w.TotalSize += uint64(fi.Size())
			}
		}
		return nil
	})

	slices.SortFunc(w.Files, func(a, b fileAndSize) bool {
		return a.Size > b.Size
	})

	return w, err
}

func makeSizeError(w workspaceSizeReport) error {
	fileList := w.Files

	var description string
	if len(fileList) > 0 {
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
		humanize.Bytes(w.TotalSize), humanize.Bytes(maxExpectedWorkspaceSize), description)
}
