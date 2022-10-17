// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devhost

import (
	"context"
	"errors"
	"io/fs"
	"reflect"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/parsing"
)

// Delete deletes the devhost filename. We gracefully handle
// the case where the file does not exist and return errors
// only if we fail to remove a valid file.
func Delete(ctx context.Context, root *parsing.Root) error {
	fsys := root.ReadWriteFS()

	// Ignore if the devhost file is not found.
	_, err := fsys.Open(DevHostFilename)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	if rmfs, ok := fsys.(fnfs.WriteFS); ok {
		return rmfs.Remove(DevHostFilename)
	}

	return fnerrors.BadInputError("workspace root is a non-writable filesystem of type %q", reflect.TypeOf(fsys))
}
