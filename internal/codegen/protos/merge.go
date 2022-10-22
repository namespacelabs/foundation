// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package protos

import (
	"strings"

	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

// Merge produces a filename-only merge of all provided files. It is the caller's
// responsibility to make sure the contents of the protos are consistent and
// mergeable.
func Merge(files ...*FileDescriptorSetAndDeps) (*FileDescriptorSetAndDeps, error) {
	filemap := map[string]*descriptorpb.FileDescriptorProto{}
	depmap := map[string]*descriptorpb.FileDescriptorProto{}

	merged := &FileDescriptorSetAndDeps{}
	for _, file := range files {
		for _, f := range file.File {
			if previous, ok := filemap[f.GetName()]; ok {
				if !proto.Equal(previous, f) {
					return nil, fnerrors.BadInputError("%s: incompatible protos", f.GetName())
				}
			} else {
				merged.File = append(merged.File, f)
				filemap[f.GetName()] = f
			}
		}
	}

	for _, file := range files {
		for _, dep := range file.Dependency {
			if _, ok := filemap[dep.GetName()]; ok {
				continue
			}

			if previous, ok := depmap[dep.GetName()]; ok {
				if !proto.Equal(previous, dep) {
					return nil, fnerrors.BadInputError("%s: incompatible dependency", dep.GetName())
				}
			} else {
				merged.Dependency = append(merged.Dependency, dep)
				depmap[dep.GetName()] = dep
			}
		}
	}

	slices.SortFunc(merged.File, func(a, b *descriptorpb.FileDescriptorProto) bool {
		return strings.Compare(a.GetName(), b.GetName()) < 0
	})

	slices.SortFunc(merged.Dependency, func(a, b *descriptorpb.FileDescriptorProto) bool {
		return strings.Compare(a.GetName(), b.GetName()) < 0
	})

	return merged, nil
}
