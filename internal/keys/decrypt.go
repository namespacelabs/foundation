// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package keys

import (
	"context"
	"io"
	"io/fs"
	"strings"

	"filippo.io/age"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const SnapshotKeys = "fn.keys"

type Reader interface {
	io.Reader
	io.ReaderAt
}

var ErrKeyGen = fnerrors.UsageError("Please run `ns keys generate` to generate a new identity.", "Decryption requires that at least one identity to be configured.")

func Decrypt(ctx context.Context, keyDir fs.FS, src io.Reader) ([]byte, error) {
	if keyDir == nil {
		return nil, ErrKeyGen
	}

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
		if _, ok := err.(*age.NoIdentityMatchError); ok {
			var recipients []string
			for _, x := range identities {
				if id, ok := x.(*age.X25519Identity); ok {
					recipients = append(recipients, id.Recipient().String())
				}
			}

			return nil, fnerrors.New("failed to decrypt: no identity matched (had %s)", strings.Join(recipients, ", "))
		}

		return nil, err
	}

	decryptedContents, err := io.ReadAll(decrypted)
	if err != nil {
		return nil, err
	}

	return decryptedContents, nil
}
