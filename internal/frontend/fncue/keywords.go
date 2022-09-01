// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package fncue

const (
	InputKeyword      = "input"
	AllocKeyword      = "alloc"
	PackageIKW        = "package"
	ServerDepIKw      = "server_dep"
	ImageIKw          = "image"
	EnvIKw            = "env"
	VCSIKw            = "vcs"
	WorkspaceIKw      = "workspace"
	ProtoloadIKw      = "protoload"
	ServerPortAllocKw = "port"
	ResourceIKw       = "resource"
)

var (
	knownInputs = []string{ServerDepIKw, ImageIKw, EnvIKw, WorkspaceIKw, ProtoloadIKw, ResourceIKw, PackageIKW, VCSIKw}
	knownAllocs = []string{ServerPortAllocKw}
)
