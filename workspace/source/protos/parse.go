// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"io"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/uniquestrings"

	// Include descriptors for generated googleapis.
	_ "google.golang.org/genproto/googleapis/api/annotations"
)

func Parse(fsys fs.FS, files []string) (*FileDescriptorSetAndDeps, error) {
	p := protoparse.Parser{
		ImportPaths:           []string{"."},
		IncludeSourceCodeInfo: true,
		Accessor: func(filename string) (io.ReadCloser, error) {
			return fsys.Open(filename)
		},
		// This allows imports to be resolved via loading linked-in descriptors.
		LookupImport: desc.LoadFileDescriptor,
	}

	files, err := expandProtoList(fsys, files)
	if err != nil {
		return nil, err
	}

	descs, err := p.ParseFiles(files...)
	if err != nil {
		return nil, fnerrors.BadInputError("proto parse failed of %q: %w", files, err)
	}

	protos := protoList{refs: map[string]*dpb.FileDescriptorProto{}}
	protoDeps := protoList{refs: map[string]*dpb.FileDescriptorProto{}}

	for _, desc := range descs {
		protos.add(desc.AsFileDescriptorProto())
	}

	for _, desc := range descs {
		protoDeps.addDeps(&protos, desc)
	}

	return &FileDescriptorSetAndDeps{File: protos.sorted(), Dependency: protoDeps.sorted()}, nil
}

type Location interface {
	Rel(...string) string
}

func ParseAtLocation(fsys fs.FS, loc Location, files []string) (*FileDescriptorSetAndDeps, error) {
	var protosrcs uniquestrings.List

	for _, srcfile := range files {
		protosrcs.Add(loc.Rel(srcfile))
	}

	return Parse(fsys, protosrcs.Strings())
}

type protoList struct {
	protos []*dpb.FileDescriptorProto
	refs   map[string]*dpb.FileDescriptorProto // map filename -> FileDescriptorProto to keep track of dups
}

func (pl *protoList) add(fproto *dpb.FileDescriptorProto) {
	if _, has := pl.refs[fproto.GetName()]; !has {
		pl.refs[fproto.GetName()] = fproto
		pl.protos = append(pl.protos, fproto)
	}
}

func (pl *protoList) addDeps(check *protoList, desc *desc.FileDescriptor) {
	for _, dep := range desc.GetDependencies() {
		x := dep.AsFileDescriptorProto()
		if !check.has(x) {
			pl.add(dep.AsFileDescriptorProto())
		}

		pl.addDeps(check, dep)
	}
}

func (pl *protoList) has(fproto *dpb.FileDescriptorProto) bool {
	_, has := pl.refs[fproto.GetName()]
	return has
}

func (pl *protoList) sorted() []*dpb.FileDescriptorProto {
	sort.Slice(pl.protos, func(i, j int) bool {
		return strings.Compare(pl.protos[i].GetName(), pl.protos[j].GetName()) < 0
	})
	return pl.protos
}

func expandProtoList(fsys fs.FS, files []string) ([]string, error) {
	var ret []string
	for _, f := range files {
		st, err := fs.Stat(fsys, f)
		if err != nil {
			return nil, err
		}

		if st.IsDir() {
			dirents, err := fs.ReadDir(fsys, f)
			if err != nil {
				return nil, err
			}

			var children []string
			for _, dirent := range dirents {
				if dirent.IsDir() || filepath.Ext(dirent.Name()) == ".proto" {
					children = append(children, filepath.Join(f, dirent.Name()))
				}
			}

			further, err := expandProtoList(fsys, children)
			if err != nil {
				return nil, err
			}

			ret = append(ret, further...)
		} else {
			ret = append(ret, f)
		}
	}
	return ret, nil
}
