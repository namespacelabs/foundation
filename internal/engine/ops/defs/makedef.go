// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package defs

import "namespacelabs.dev/foundation/schema"

type MakeDefinition interface {
	ToDefinition(...schema.PackageName) (*schema.Definition, error)
}
