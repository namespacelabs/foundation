// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package keys

import (
	"bytes"
	"context"
	"io"
	"io/fs"

	"filippo.io/age"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
)

type Reader interface {
	io.Reader
	io.ReaderAt
}

var ErrKeyGen = fnerrors.UsageError("Please run `fn keys generate` to generate a new identity.", "Decryption requires that at least one identity to be configured.")

func Decrypt(ctx context.Context, keyDir fs.FS, src io.Reader) ([]byte, error) {
	var identities []age.Identity
	if err := Visit(ctx, keyDir, func(xi *age.X25519Identity) error {
		identities = append(identities, xi)
		return nil
	}); err != nil {
		return nil, err
	}

	if len(identities) == 0 {
		return nil, ErrKeyGen
	}

	decrypted, err := age.Decrypt(src, identities...)
	if err != nil {
		return nil, err
	}

	decryptedContents, err := io.ReadAll(decrypted)
	if err != nil {
		return nil, err
	}

	return decryptedContents, nil
}

func DecryptAsFS(ctx context.Context, keyDir fs.FS, archive io.Reader) (fs.FS, error) {
	decrypted, err := Decrypt(ctx, keyDir, archive)
	if err != nil {
		return nil, err
	}

	return tarfs.FS{
		TarStream: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(decrypted)), nil
		},
	}, nil
}
