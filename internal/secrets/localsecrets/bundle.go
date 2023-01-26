// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package localsecrets

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"os"

	"filippo.io/age"
	"github.com/muesli/reflow/wordwrap"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/maketarfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/keys"
)

const (
	UserBundleName      = "user.secrets"
	WorkspaceBundleName = "workspace.secrets"
	ServerBundleName    = "server.secrets"

	guardBegin = "BEGIN FOUNDATION SECRET BUNDLE"
	guardEnd   = "END FOUNDATION SECRET BUNDLE"
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
	filename     string
	m            *ValueDatabase
	files        *memfs.FS
	wasEncrypted bool
}

func (b *Bundle) Readers() []*Manifest_Reader {
	return b.m.Reader
}

func (b *Bundle) Definitions() []*Manifest_Definition {
	return b.m.Definition
}

func (b *Bundle) Set(k *ValueKey, value []byte) {
	var hasDef bool

	for _, sec := range b.values {
		for _, v := range sec.m.Value {
			if equalKey(v.Key, k) {
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
			Key:   k,
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
				// Delete all {packageName, key} pairs, regardless of environment.
				return e.Key.GetPackageName() == packageName && e.Key.GetKey() == key
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

func (b *Bundle) Lookup(ctx context.Context, key *ValueKey) ([]byte, error) {
	sel, v := b.match(key)
	if v == nil {
		return nil, nil
	}

	if v.FromPath != "" {
		return fs.ReadFile(sel.files, v.FromPath)
	}

	return v.Value, nil
}

func (b *Bundle) WasEncrypted(key *ValueKey) (bool, bool) {
	v, _ := b.match(key)
	if v == nil {
		return false, false
	}
	return v.wasEncrypted, true
}

func (b *Bundle) match(key *ValueKey) (*valueDatabase, *ValueDatabase_Value) {
	var sel *valueDatabase
	var nonSpecific *ValueDatabase_Value

	for _, sec := range b.values {
		for _, v := range sec.m.Value {
			if equalKey(v.Key, key) {
				// If the key is not bound to an environment, keep looking as there may be one that is.
				if v.Key.EnvironmentName == "" {
					sel = &sec
					nonSpecific = v
				} else if v.Key.EnvironmentName == key.EnvironmentName {
					return &sec, v
				}
			}
		}
	}

	return sel, nonSpecific
}

type LookupResult struct {
	Key   *ValueKey
	Value []byte
}

func (b *Bundle) LookupValues(ctx context.Context, key *ValueKey) ([]LookupResult, error) {
	var results []LookupResult
	for _, sec := range b.values {
		for _, v := range sec.m.Value {
			if key.PackageName == v.Key.PackageName && key.Key == v.Key.Key && (key.EnvironmentName == "" || key.EnvironmentName == v.Key.EnvironmentName) {
				contents := v.Value
				if v.FromPath != "" {
					var err error
					contents, err = fs.ReadFile(sec.files, v.FromPath)
					if err != nil {
						return nil, err
					}
				}

				results = append(results, LookupResult{v.Key, contents})
			}
		}
	}

	return results, nil
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

func (b *Bundle) SetReaders(pubkeys []string) error {
	if len(pubkeys) == 0 {
		return fnerrors.New("can't reset the readers to an empty list")
	}

	var keys []*age.X25519Recipient

	for _, pubkey := range pubkeys {
		xid, err := age.ParseX25519Recipient(pubkey)
		if err != nil {
			return fnerrors.BadInputError("bad receipient %q: %w", pubkey, err)
		}
		keys = append(keys, xid)
	}

	b.m.Reader = nil
	for _, xid := range keys {
		b.m.Reader = append(b.m.Reader, &Manifest_Reader{PublicKey: xid.String()})
	}

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
	ww := wordwrap.NewWriter(80)

	fmt.Fprintf(ww, "This is a secrets bundle managed by Namespace. You can use `ns secrets` to modify this file.\n\nNOTE: Any changes made before %q are ignored.\n\n", guardBegin)

	if err := ww.Close(); err != nil {
		return err
	}

	if _, err := w.Write(ww.Bytes()); err != nil {
		return err
	}

	b.DescribeTo(w)
	fmt.Fprintln(w)

	fmt.Fprintf(w, "-----%s-----\n", guardBegin)

	var serialized memfs.FS

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
				return fnerrors.InternalError("failed to close encrypted bundle: %w", err)
			}
		} else {
			if err := maketarfs.TarFS(ctx, &buf, encFS, nil, nil); err != nil {
				return fnerrors.InternalError("failed to create encrypted bundle: %w", err)
			}
		}

		serialized.Add(enc.filename, buf.Bytes())
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := maketarfs.TarFS(ctx, gz, &serialized, nil, nil); err != nil {
		return err
	}

	if err := gz.Close(); err != nil {
		return err
	}

	if err := OutputBase64(w, buf.Bytes()); err != nil {
		return err
	}

	fmt.Fprintf(w, "-----%s-----\n", guardEnd)

	return err
}

func OutputBase64(w io.Writer, buf []byte) error {
	var breaker lineBreaker
	breaker.out = w

	b64 := base64.NewEncoder(base64.StdEncoding, &breaker)
	if _, err := b64.Write(buf); err != nil {
		return err
	}

	if err := b64.Close(); err != nil {
		return err
	}

	if err := breaker.Close(); err != nil {
		return err
	}

	return nil
}

func (b *Bundle) DescribeTo(out io.Writer) {
	switch len(b.Readers()) {
	case 0:
		fmt.Fprintln(out, "No readers.")

	default:
		fmt.Fprintln(out, "Readers:")
		for _, r := range b.Readers() {
			fmt.Fprintf(out, "  %s", r.PublicKey)
			if r.Description != "" {
				fmt.Fprintf(out, "  # %s", r.Description)
			}
			fmt.Fprintln(out)
		}
	}

	switch len(b.Definitions()) {
	case 0:
		fmt.Fprintln(out, "No definitions.")

	default:
		fmt.Fprintln(out, "Definitions:")
		for _, def := range b.Definitions() {
			fmt.Fprintf(out, "  ")
			DescribeKey(out, def.Key)
			if wasEncrypted, found := b.WasEncrypted(def.Key); found && !wasEncrypted {
				fmt.Fprintf(out, " (unencrypted)")
			}
			fmt.Fprintln(out)
		}
	}
}

func DescribeKey(out io.Writer, key *ValueKey) {
	fmt.Fprintf(out, "%s:%s", key.PackageName, key.Key)
	if key.EnvironmentName != "" {
		fmt.Fprintf(out, " (%s)", key.EnvironmentName)
	}
}

func equalKey(a, b *ValueKey) bool {
	return a.PackageName == b.PackageName && a.Key == b.Key && a.EnvironmentName == b.EnvironmentName
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

func LoadBundle(ctx context.Context, keyDir fs.FS, raw []byte) (*Bundle, error) {
	contents, err := detect(raw)
	if err != nil {
		return nil, err
	}

	fsys := tarfs.FS{
		TarStream: func() (io.ReadCloser, error) {
			return gzip.NewReader(bytes.NewReader(contents))
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
			archiveBytes, err = io.ReadAll(encFile)
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
			filename:     enc.Filename,
			m:            encm,
			files:        &memfs.FS{},
			wasEncrypted: !enc.RawText,
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

func detect(raw []byte) ([]byte, error) {
	guard := []byte(fmt.Sprintf("-----%s-----\n", guardBegin))
	idx := bytes.Index(raw, guard)
	if idx < 0 {
		return nil, fnerrors.New("invalid bundle: missing begin guard")
	}

	rawLeft := raw[(idx + len(guard)):]

	endIdx := bytes.Index(rawLeft, []byte(fmt.Sprintf("-----%s-----", guardEnd)))
	if endIdx < 0 {
		return nil, fnerrors.New("invalid bundle: missing end guard")
	}

	return io.ReadAll(base64.NewDecoder(base64.StdEncoding, bytes.NewReader(rawLeft[:endIdx])))
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

// From Go's pem library.
const blockLineLength = 80

type lineBreaker struct {
	line [blockLineLength]byte
	used int
	out  io.Writer
}

var nl = []byte{'\n'}

func (l *lineBreaker) Write(b []byte) (n int, err error) {
	if l.used+len(b) < blockLineLength {
		copy(l.line[l.used:], b)
		l.used += len(b)
		return len(b), nil
	}

	n, err = l.out.Write(l.line[0:l.used])
	if err != nil {
		return
	}
	excess := blockLineLength - l.used
	l.used = 0

	n, err = l.out.Write(b[0:excess])
	if err != nil {
		return
	}

	n, err = l.out.Write(nl)
	if err != nil {
		return
	}

	return l.Write(b[excess:])
}

func (l *lineBreaker) Close() (err error) {
	if l.used > 0 {
		_, err = l.out.Write(l.line[0:l.used])
		if err != nil {
			return
		}
		_, err = l.out.Write(nl)
	}

	return
}
