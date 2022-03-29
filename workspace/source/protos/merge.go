// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import (
	"sort"
	"strings"

	"google.golang.org/protobuf/types/descriptorpb"
)

// Merge produces a filename-only merge of all provided files. It is the caller's
// responsibility to make sure the contents of the protos are consistent and
// mergeable.
func Merge(files ...*FileDescriptorSetAndDeps) *FileDescriptorSetAndDeps {
	filemap := map[string]*descriptorpb.FileDescriptorProto{}
	depmap := map[string]*descriptorpb.FileDescriptorProto{}

	for _, file := range files {
		for _, f := range file.File {
			filemap[f.GetName()] = f
			delete(depmap, f.GetName())
		}
		for _, dep := range file.Dependency {
			if _, has := filemap[dep.GetName()]; !has {
				depmap[dep.GetName()] = dep
			}
		}
	}

	merged := &FileDescriptorSetAndDeps{}
	for _, f := range filemap {
		merged.File = append(merged.File, f)
	}
	for _, dep := range depmap {
		merged.Dependency = append(merged.Dependency, dep)
	}

	sort.Slice(merged.File, func(i, j int) bool {
		return strings.Compare(merged.File[i].GetName(), merged.File[j].GetName()) < 0
	})
	sort.Slice(merged.Dependency, func(i, j int) bool {
		return strings.Compare(merged.Dependency[i].GetName(), merged.Dependency[j].GetName()) < 0
	})

	return merged
}