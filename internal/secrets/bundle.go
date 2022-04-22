// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package secrets

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"

	"filippo.io/age"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/keys"
)

// A secrets bundle is a tar file which includes:
// - README.txt: A disclaimer file with information about the contents.
// - manifest.json: A list of defined keys, and receipients.
// - encrypted.age: An age-encrypted tar file with the secret values, or secret files.

type Bundle struct {
	m *Manifest

	values []valueDatabase
}

type valueDatabase struct {
	filename string
	m        *ValueDatabase
	files    *memfs.FS
}

func (b *Bundle) Readers() []*Manifest_Reader {
	return b.m.Reader
}

func (b *Bundle) Definitions() []*Manifest_Definition {
	return b.m.Definition
}

func (b *Bundle) Set(packageName, key string, value []byte) {
	var hasDef bool

	for _, sec := range b.values {
		for _, v := range sec.m.Value {
			if isKey(v.Key, packageName, key) {
				v.Value = value
				v.FromPath = ""
				hasDef = true
				break
			}
		}
	}

	if !hasDef {
		if len(b.values) == 0 {
			b.values = append(b.values, valueDatabase{
				filename: fmt.Sprintf("values/%d.values", len(b.values)),
				m:        &ValueDatabase{},
				files:    &memfs.FS{},
			})
		}

		enc := b.values[len(b.values)-1]
		enc.m.Value = append(enc.m.Value, &ValueDatabase_Value{
			Key: &ValueKey{
				PackageName: packageName,
				Key:         key,
			},
			Value: value,
		})
	}

	// Always re-generate definitions.
	b.regen()
}

func (b *Bundle) Delete(packageName, key string) bool {
	var deleted int
	for _, sec := range b.values {
		for {
			index := slices.IndexFunc(sec.m.Value, func(e *ValueDatabase_Value) bool {
				return isKey(e.Key, packageName, key)
			})
			if index < 0 {
				break
			}

			sec.m.Value = slices.Delete(sec.m.Value, index, index+1)
			deleted++
		}
	}

	if deleted > 0 {
		b.regen()
		return true
	}

	return false
}

func (b *Bundle) Lookup(ctx context.Context, packageName, key string) ([]byte, error) {
	for _, sec := range b.values {
		for _, v := range sec.m.Value {
			if isKey(v.Key, packageName, key) {
				if v.FromPath != "" {
					return fs.ReadFile(sec.files, v.FromPath)
				}

				return v.Value, nil
			}
		}
	}

	return nil, nil
}

func (b *Bundle) EnsureReader(pubkey string) error {
	xid, err := age.ParseX25519Recipient(pubkey)
	if err != nil {
		return fnerrors.BadInputError("bad receipient: %w", err)
	}

	for _, r := range b.m.Reader {
		if r.PublicKey == xid.String() {
			return nil
		}
	}

	b.m.Reader = append(b.m.Reader, &Manifest_Reader{PublicKey: xid.String()})
	return nil
}

func (b *Bundle) regen() {
	b.m.Definition = nil
	for _, enc := range b.values {
		for _, v := range enc.m.Value {
			b.m.Definition = append(b.m.Definition, &Manifest_Definition{
				Key: v.Key,
			})
		}
	}
}

func (b *Bundle) SerializeTo(ctx context.Context, w io.Writer, encrypt bool) error {
	var serialized memfs.FS

	serialized.Add("README.txt", []byte("This is a secret bundle managed by Foundation. A list of included secrets is always visible, but their values are encrypted."))

	var recipients []age.Recipient
	for _, reader := range b.m.Reader {
		xid, err := age.ParseX25519Recipient(reader.PublicKey)
		if err != nil {
			return fnerrors.BadInputError("invalid bundle: bad receipient: %w", err)
		}

		recipients = append(recipients, xid)
	}

	m := &Manifest{
		Definition: b.m.Definition,
		Reader:     b.m.Reader,
	}

	for _, enc := range b.values {
		m.Values = append(m.Values, &Manifest_BundleReference{
			Filename: enc.filename,
			RawText:  !encrypt,
		})
	}

	manifestBytes, err := protojson.Marshal(m)
	if err != nil {
		return fnerrors.InternalError("failed to serialize manifest: %w", err)
	}

	serialized.Add("manifest.json", manifestBytes)

	for _, enc := range b.values {
		manifestBytes, err := protojson.Marshal(enc.m)
		if err != nil {
			return fnerrors.InternalError("failed to serialize manifest: %w", err)
		}

		var encFS *memfs.FS
		if enc.files != nil {
			encFS = enc.files.Clone().(*memfs.FS)
		} else {
			encFS = &memfs.FS{}
		}

		encFS.Add("manifest.json", manifestBytes)

		var buf bytes.Buffer
		if encrypt {
			encryptedWriter, encErr := age.Encrypt(&buf, recipients...)
			if encErr != nil {
				return fnerrors.InternalError("failed to encrypt values: %w", err)
			}

			if err := maketarfs.TarFS(ctx, encryptedWriter, encFS, nil, nil); err != nil {
				return fnerrors.InternalError("failed to create encrypted bundle: %w", err)
			}

			if err := encryptedWriter.Close(); err != nil {
				return fnerrors.InternalError("failed to encrypted bundle: %w", err)
			}
		} else {
			if err := maketarfs.TarFS(ctx, &buf, encFS, nil, nil); err != nil {
				return fnerrors.InternalError("failed to create encrypted bundle: %w", err)
			}
		}

		serialized.Add(enc.filename, buf.Bytes())
	}

	return maketarfs.TarFS(ctx, w, &serialized, nil, nil)
}

func isKey(k *ValueKey, packageName, key string) bool {
	return k.PackageName == packageName && k.Key == key && k.SecondaryKey == ""
}

func NewBundle(ctx context.Context, keyID string) (*Bundle, error) {
	identity, err := keys.Select(ctx, keyID)
	if err != nil {
		return nil, err
	}

	return &Bundle{
		m: &Manifest{
			Reader: []*Manifest_Reader{
				{PublicKey: identity.Recipient().String()},
			},
		},
	}, nil
}

func LoadBundle(ctx context.Context, keyDir fs.FS, contents []byte) (*Bundle, error) {
	fsys := tarfs.FS{
		TarStream: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(contents)), nil
		},
	}

	m := &Manifest{}
	if err := readManifest("bundle", fsys, m); err != nil {
		return nil, err
	}

	bundle := &Bundle{m: m}

	for _, enc := range m.Values {
		encFile, err := fsys.Open(enc.Filename)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fnerrors.BadInputError("invalid bundle: encrypted values %q are missing", enc.Filename)
			}
			return nil, err
		}

		defer encFile.Close()

		var archiveBytes []byte
		if !enc.RawText {
			archiveBytes, err = keys.Decrypt(ctx, keyDir, encFile)
			if err != nil {
				return nil, err
			}
		} else {
			archiveBytes, err = ioutil.ReadAll(encFile)
			if err != nil {
				return nil, err
			}
		}

		encFS := tarfs.FS{
			TarStream: func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(archiveBytes)), nil
			},
		}

		encm := &ValueDatabase{}
		if err := readManifest("value bundle", encFS, encm); err != nil {
			return nil, err
		}

		encbundle := valueDatabase{
			filename: enc.Filename,
			m:        encm,
			files:    &memfs.FS{},
		}

		for _, value := range encm.Value {
			if value.FromPath != "" {
				encrypted, err := fs.ReadFile(encFS, value.FromPath)
				if err != nil {
					return nil, fnerrors.BadInputError("invalid bundle: missing encrypted value: %w", err)
				}

				encbundle.files.Add(value.FromPath, encrypted)
			}
		}

		bundle.values = append(bundle.values, encbundle)
	}

	return bundle, nil
}

func readManifest(what string, fsys fs.FS, m proto.Message) error {
	manifestBytes, err := fs.ReadFile(fsys, "manifest.json")
	if err != nil {
		if os.IsNotExist(err) {
			return fnerrors.BadInputError("invalid %s: manifest.json is missing", what)
		}
		return err
	}

	if err := protojson.Unmarshal(manifestBytes, m); err != nil {
		return fnerrors.BadInputError("invalid %s: manifest.json is invalid: %w", what, err)
	}

	return nil
}
