// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package opaque

import "namespacelabs.dev/foundation/schema"

const (
	forceProd = false
)

func UseDevBuild(env *schema.Environment) bool {
	return !forceProd && env.Purpose == schema.Environment_DEVELOPMENT
}
