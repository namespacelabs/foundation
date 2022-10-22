// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package protos

import (
	"path/filepath"
	"strings"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/schema"
)

const FoundationTypeUrlBaseSlash = "type.foundation.namespacelabs.dev/"

type TypeReference struct {
	Package   schema.PackageName
	ProtoType string
	Builtin   bool
}

func Ref(dep *anypb.Any) *TypeReference {
	if t := strings.TrimPrefix(dep.GetTypeUrl(), "type.googleapis.com/"); t != dep.GetTypeUrl() {
		return &TypeReference{
			ProtoType: t,
			Builtin:   true,
		}
	}

	if t := strings.TrimPrefix(dep.GetTypeUrl(), FoundationTypeUrlBaseSlash); t != dep.GetTypeUrl() {
		return &TypeReference{
			Package:   schema.PackageName(filepath.Dir(t)),
			ProtoType: filepath.Base(t),
		}
	}

	return nil
}
