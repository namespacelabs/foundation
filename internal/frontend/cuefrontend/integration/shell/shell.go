// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package shell

import (
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/entity"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/helpers"
	"namespacelabs.dev/foundation/schema"
)

func NewParser() entity.EntityParser {
	return &helpers.SimpleJsonParser[*schema.ShellIntegration]{
		SyntaxUrl:      "namespace.so/from-shell",
		SyntaxShortcut: "shell",
	}
}
