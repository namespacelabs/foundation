// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package keys

import (
	"bufio"
	"context"
	"io"
	"io/fs"
	"strings"

	"filippo.io/age"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
)

const (
	EncryptedFile = "contents.tar.age"
	keyListFile   = "contents.tar.keys"
)

var internalFiles = []string{EncryptedFile, keyListFile}

func EncryptLocal(ctx context.Context, dst fnfs.LocalFS, src fs.ReadDirFS) error {
	return Encrypt(dst, func(w io.Writer) error {
		return maketarfs.TarFS(ctx, w, src, nil, internalFiles)
	})
}

func Reencrypt(ctx context.Context, l fnfs.LocalFS) error {
	keyDir, err := KeysDir()
	if err != nil {
		return err
	}
	var identities []age.Identity
	if err := Visit(ctx, keyDir, func(xid *age.X25519Identity) error {
		identities = append(identities, xid)
		return nil
	}); err != nil {
		return err
	}

	existingContents, err := l.Open(EncryptedFile)
	if err != nil {
		return fnerrors.BadInputError("failed to open encrypted file: %w", err)
	}

	decrypted, err := Decrypt(ctx, keyDir, existingContents)
	if err != nil {
		return fnerrors.BadInputError("failed to decrypt: %w", err)
	}

	return Encrypt(l, func(w io.Writer) error {
		_, err := w.Write(decrypted)
		return err
	})
}

func Encrypt(l fnfs.LocalFS, writeBack func(w io.Writer) error) error {
	keyList, encErr := l.Open(keyListFile)
	if encErr != nil {
		return encErr
	}
	defer keyList.Close()

	var recipients []age.Recipient

	scanner := bufio.NewScanner(keyList)
	for scanner.Scan() {
		line := scanner.Text()
		spaceless := strings.TrimSpace(line)
		if strings.HasPrefix(spaceless, "#") {
			continue
		}

		xid, err := age.ParseX25519Recipient(spaceless)
		if err != nil {
			return err
		}
		recipients = append(recipients, xid)
	}

	w, encErr := l.OpenWrite(EncryptedFile, 0644)
	if encErr != nil {
		return encErr
	}

	encryptedWriter, encErr := age.Encrypt(w, recipients...)
	if encErr == nil {
		encErr = writeBack(encryptedWriter)
		encCloseErr := encryptedWriter.Close()
		if encErr == nil {
			encErr = encCloseErr
		}
	}

	closeErr := w.Close()
	if encErr != nil {
		return encErr
	}

	return closeErr
}