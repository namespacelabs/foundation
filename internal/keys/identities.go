// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package keys

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"filippo.io/age"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/workspace/dirs"
)

func Visit(ctx context.Context, keysDir fs.ReadDirFS, callback func(*age.X25519Identity) error) error {
	return fnfs.VisitFiles(ctx, keysDir, func(path string, contents []byte, dirent fs.DirEntry) error {
		if filepath.Ext(path) != ".txt" {
			return nil
		}

		xid, err := validateKey(age.ParseIdentities(bytes.NewReader(contents)))
		if err != nil {
			return fnerrors.BadInputError("%s: %w", path, err)
		}

		if err := callback(xid); err != nil {
			return err
		}

		return nil
	})
}

func validateKey(xids []age.Identity, err error) (*age.X25519Identity, error) {
	if len(xids) != 1 {
		return nil, fnerrors.BadInputError("expected one identify, saw %d", len(xids))
	}

	id := xids[0]
	if xid, ok := id.(*age.X25519Identity); ok {
		return xid, nil
	} else {
		return nil, fnerrors.BadInputError("expected x25519 identify")
	}
}

func Key(key string) (age.Identity, error) {
	cfg, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	keyFile := filepath.Join(cfg, "keys", key+".txt")
	f, err := os.Open(keyFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	xid, err := validateKey(age.ParseIdentities(f))
	if err != nil {
		return nil, fnerrors.BadInputError("%s: %w", keyFile, err)
	}

	return xid, nil
}

func Collect(ctx context.Context) (*memfs.FS, error) {
	cfg, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	var inmem memfs.FS

	keysDir := filepath.Join(cfg, "keys")
	if _, err := os.Stat(keysDir); os.IsNotExist(err) {
		return &inmem, nil
	}

	fsys := fnfs.Local(keysDir)

	err = Visit(ctx, fsys, func(xid *age.X25519Identity) error {
		return fnfs.WriteFile(ctx, &inmem, fmt.Sprintf("%s.txt", xid.Recipient()), []byte(xid.String()), 0600)
	})

	return &inmem, err
}