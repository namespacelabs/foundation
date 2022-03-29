// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package protos

import dpb "github.com/golang/protobuf/protoc-gen-go/descriptor"

func (fds *FileDescriptorSetAndDeps) AsFileDescriptorSet() *dpb.FileDescriptorSet {
	return &dpb.FileDescriptorSet{
		File: append(append([]*dpb.FileDescriptorProto{}, fds.File...), fds.Dependency...),
	}
}