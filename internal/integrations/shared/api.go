// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package shared

import (
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ProtoTypeKind int32

const (
	ProtoMessage ProtoTypeKind = iota
	ProtoService
)

type ProtoTypeData struct {
	Name           string
	SourceFileName string
	Location       pkggraph.Location
	// Distinguishing between message and service types because they need to be imported from different files in node.js
	Kind ProtoTypeKind
}
