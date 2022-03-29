// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package memfs

import (
	"io/fs"
	"path/filepath"
	"time"
)

const memFileMode = 0644
const memDirMode = 0755

type fileDirent struct {
	Path        string
	ContentSize int64
	FileMode    fs.FileMode
}

var _ fs.FileInfo = fileDirent{}
var _ fs.DirEntry = fileDirent{}

func (f fileDirent) Name() string { return filepath.Base(f.Path) }

func (f fileDirent) IsDir() bool { return false }

func (f fileDirent) Type() fs.FileMode { return 0 /*Regular*/ }

func (f fileDirent) Info() (fs.FileInfo, error) {
	return f, nil
}

func (f fileDirent) Size() int64        { return f.ContentSize }
func (f fileDirent) Mode() fs.FileMode  { return f.FileMode }
func (f fileDirent) ModTime() time.Time { return time.Unix(1, 1) }
func (f fileDirent) Sys() interface{}   { return nil }

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