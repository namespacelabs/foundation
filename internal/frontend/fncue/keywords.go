// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fncue

const (
	InputKeyword      = "input"
	AllocKeyword      = "alloc"
	PackageIKW        = "package"
	PackageRefIKW     = "package_ref"
	ServerDepIKw      = "server_dep"
	ImageIKw          = "image"
	VCSIKw            = "vcs"
	WorkspaceIKw      = "workspace"
	ProtoloadIKw      = "protoload"
	ServerPortAllocKw = "port"
	ResourceIKw       = "resource"
)

var (
	knownInputs = []string{ServerDepIKw, ImageIKw, WorkspaceIKw, ProtoloadIKw, ResourceIKw, PackageIKW, PackageRefIKW, VCSIKw}
	knownAllocs = []string{ServerPortAllocKw}
)
