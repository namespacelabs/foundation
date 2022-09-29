// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package opaque

import (
	"namespacelabs.dev/foundation/languages"
	"namespacelabs.dev/foundation/languages/opaque"
	"namespacelabs.dev/foundation/schema"
)

func Register() {
	languages.Register(schema.Framework_OPAQUE_NODEJS, impl{})
}

type impl struct {
	opaque.OpaqueIntegration
}
