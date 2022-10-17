// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package keys

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

func KeysDir() (fnfs.LocalFS, error) {
	cfg, err := dirs.Config()
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrKeyGen
		}
		return nil, err
	}

	keysDir := filepath.Join(cfg, "keys")
	if _, err := os.Stat(keysDir); os.IsNotExist(err) {
		return nil, ErrKeyGen
	} else if err != nil {
		return nil, err
	}

	return fnfs.Local(keysDir), nil
}

func EnsureKeysDir(ctx context.Context) (fnfs.LocalFS, error) {
	cfg, err := dirs.Config()
	if err != nil {
		return nil, err
	}

	keysDir := filepath.Join(cfg, "keys")

	if st, err := os.Stat(keysDir); os.IsNotExist(err) {
		if err := os.MkdirAll(keysDir, 0700); err != nil {
			return nil, err
		}
	} else if mode := st.Mode().Perm(); mode != 0700 {
		fmt.Fprintf(console.Stderr(ctx), "%s expected permissions to be %o, saw %o\n", keysDir, 0700, mode)
	}

	return fnfs.ReadWriteLocalFS(keysDir), nil
}
