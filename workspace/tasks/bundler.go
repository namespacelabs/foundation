// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package tasks

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/workspace/dirs"
)

const (
	bundleTimeFormat      = "2006-01-02T15-04-05"
	defaultBundlesToKeep  = 10
	defaultBundleDuration = 48 * time.Hour
)

// Bundler manages creation and rolling of subdirectory `Bundle`s in a local fs.
type Bundler struct {
	// absolute path to the root directory of this local fs.
	root string
	fsys fnfs.LocalFS

	// Prefix for timestamped bundle subdirs.
	namePrefix string

	// Maximum number of bundles to retain.
	maxBundles int

	// Maximum duration for which bundles should be retained.
	maxAge time.Duration
}

// Returns a new instance of Bundler with default values.
func NewActionBundler() (*Bundler, error) {
	cacheDir, err := dirs.Cache()
	if err != nil {
		return nil, err
	}
	root := filepath.Join(cacheDir, "action-bundles")
	return &Bundler{
		root:       root,
		fsys:       fnfs.ReadWriteLocalFS(root),
		namePrefix: "actions",
		maxBundles: defaultBundlesToKeep,
		maxAge:     defaultBundleDuration,
	}, nil
}

// Returns a new Bundle with the current timestamp.
func (b *Bundler) NewBundle() (*Bundle, error) {
	t := time.Now().UTC()
	ts := t.Format(bundleTimeFormat)
	bundleDir := fmt.Sprintf("%s-%s", b.namePrefix, ts)

	if mkdirfs, ok := b.fsys.(fnfs.MkdirFS); ok {
		err := mkdirfs.MkdirAll(bundleDir, 0700)
		if err != nil {
			return nil, fnerrors.InternalError("failed to create timestamped bundle dir: %w", err)
		}
	}
	return &Bundle{
		fsys:      fnfs.ReadWriteLocalFS(filepath.Join(b.root, bundleDir)),
		Timestamp: t,
	}, nil
}

func (b *Bundler) timeFromName(bundleName string) (time.Time, error) {
	expectedPrefix := b.namePrefix + "-"
	if !strings.HasPrefix(bundleName, expectedPrefix) {
		return time.Time{}, fnerrors.BadInputError("expected prefix %q in name %q", expectedPrefix, bundleName)
	}
	ts := bundleName[len(expectedPrefix):]
	t, err := time.Parse(bundleTimeFormat, ts)
	if err != nil {
		return time.Time{}, fnerrors.BadInputError("failed to parse timestamp from name %q: %w", bundleName, err)
	}
	return t, nil
}

// Returns bundles ordered by the newest timestamp.
func (b *Bundler) ReadBundles() ([]*Bundle, error) {
	files, err := b.fsys.ReadDir(".")
	if err != nil {
		return nil, fnerrors.InternalError("failed to read bundles: %w", err)
	}
	bundles := []*Bundle{}
	for _, f := range files {
		baseName := filepath.Base(f.Name())
		t, err := b.timeFromName(baseName)
		if err != nil {
			return nil, err
		}
		bundle := &Bundle{
			fsys:      fnfs.ReadWriteLocalFS(filepath.Join(b.root, baseName)),
			Timestamp: t,
		}
		bundles = append(bundles, bundle)
	}
	sort.Sort(byFormatTime(bundles))
	return bundles, nil
}

// Removes bundles which are older than `b.maxAge` or if we exceed
// `b.maxBundles.`
func (b *Bundler) RemoveStaleBundles() error {
	if b.maxAge == 0 && b.maxBundles == 0 {
		return nil
	}
	bundles, err := b.ReadBundles()
	if err != nil {
		return fnerrors.InternalError("failed to read bundles: %w", err)
	}
	var remove []*Bundle
	if b.maxBundles > 0 && len(bundles) > b.maxBundles {
		remove = bundles[b.maxBundles:]
		bundles = bundles[:b.maxBundles]
	}
	if b.maxAge > 0 {
		cutoff := time.Now().UTC().Add(-1 * b.maxAge)
		for _, bundle := range bundles {
			if bundle.Timestamp.Before(cutoff) {
				remove = append(remove, bundle)
			}
		}
	}
	for _, bundle := range remove {
		if rmdirfs, ok := bundle.fsys.(fnfs.RmdirFS); ok {
			err := rmdirfs.RemoveAll(".")
			if err != nil {
				return fnerrors.InternalError("failed to delete bundle: %w", err)
			}
		}
	}
	return nil
}
