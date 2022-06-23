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
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/descriptorpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/uniquestrings"
	"namespacelabs.dev/foundation/workspace/dirs"

	// Include descriptors for generated googleapis.
	_ "google.golang.org/genproto/googleapis/api/annotations"
)

type ParseOpts struct {
	KnownModules []struct {
		ModuleName string
		FS         fs.FS
	}
}

func (opts ParseOpts) Parse(fsys fs.FS, files []string) (*FileDescriptorSetAndDeps, error) {
	p := protoparse.Parser{
		ImportPaths:           []string{"."},
		IncludeSourceCodeInfo: true,
		Accessor: func(filename string) (io.ReadCloser, error) {
			return fsys.Open(filename)
		},
		LookupImport: func(path string) (*desc.FileDescriptor, error) {
			// We're explicit about what protos are exposed here, as not all
			// proto tooling will handling this well.
			if filepath.Dir(path) == "google/api" {
				return desc.LoadFileDescriptor(path)
			}

			for _, known := range opts.KnownModules {
				if rel := strings.TrimPrefix(path, known.ModuleName+"/"); rel != path {
					x, err := opts.Parse(known.FS, []string{rel})
					if err != nil {
						return nil, err
					}

					return toFileDescriptor(x.File[0], x.Dependency)
				}
			}

			// It's important to return an error, so the nil value is not used.
			return nil, fnerrors.New("no such file: %s", path)
		},
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

// XXX this is O(n^2)
func toFileDescriptor(file *descriptorpb.FileDescriptorProto, deps []*descriptorpb.FileDescriptorProto) (*desc.FileDescriptor, error) {
	var parsedDeps []*desc.FileDescriptor
	for _, dep := range file.GetDependency() {
		var found *descriptorpb.FileDescriptorProto
		for _, d := range deps {
			if d.GetName() == dep {
				found = d
				break
			}
		}
		if found == nil {
			return nil, fnerrors.New("%s: missing dependency %q", file.GetName(), dep)
		}
		parsed, err := toFileDescriptor(found, deps)
		if err != nil {
			return nil, err
		}
		parsedDeps = append(parsedDeps, parsed)
	}

	return desc.CreateFileDescriptor(file, parsedDeps...)
}

type Location interface {
	Rel(...string) string
}

func (opts ParseOpts) ParseAtLocation(fsys fs.FS, loc Location, files []string) (*FileDescriptorSetAndDeps, error) {
	var protosrcs uniquestrings.List

	for _, srcfile := range files {
		protosrcs.Add(loc.Rel(srcfile))
	}

	return opts.Parse(fsys, protosrcs.Strings())
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
			if slices.Contains(dirs.DirsToExclude, st.Name()) {
				continue
			}
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
