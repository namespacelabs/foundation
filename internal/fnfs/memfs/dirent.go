// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package memfs

import (
	"io/fs"
	"path/filepath"
	"time"
)

const memFileMode = 0644
const memDirMode = 0755

type FileDirent struct {
	Path        string
	ContentSize int64
	FileMode    fs.FileMode
}

var _ fs.FileInfo = FileDirent{}
var _ fs.DirEntry = FileDirent{}

func (f FileDirent) Name() string { return filepath.Base(f.Path) }

func (f FileDirent) IsDir() bool { return false }

func (f FileDirent) Type() fs.FileMode { return 0 /*Regular*/ }

func (f FileDirent) Info() (fs.FileInfo, error) {
	return f, nil
}

func (f FileDirent) Size() int64        { return f.ContentSize }
func (f FileDirent) Mode() fs.FileMode  { return f.FileMode }
func (f FileDirent) ModTime() time.Time { return time.Unix(1, 1) }
func (f FileDirent) Sys() interface{}   { return nil }

type dirDirent struct {
	Basename string
	DirMode  fs.FileMode
}

var _ fs.FileInfo = dirDirent{}
var _ fs.DirEntry = dirDirent{}

func (f dirDirent) Name() string { return f.Basename }

func (f dirDirent) IsDir() bool { return true }

func (f dirDirent) Type() fs.FileMode { return fs.ModeDir }

func (f dirDirent) Info() (fs.FileInfo, error) {
	return f, nil
}

func (f dirDirent) Size() int64        { return 0 }
func (f dirDirent) Mode() fs.FileMode  { return f.DirMode }
func (f dirDirent) ModTime() time.Time { return time.Unix(1, 1) }
func (f dirDirent) Sys() interface{}   { return nil }
