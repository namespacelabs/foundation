// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package controller

import (
	"strings"

	"namespacelabs.dev/foundation/schema"
)

const (
	ctrlPackage = "namespacelabs.dev/foundation/std/runtime/kubernetes/controller"
)

func IsController(pkg schema.PackageName) bool {
	return strings.HasPrefix(pkg.String(), ctrlPackage)
}
