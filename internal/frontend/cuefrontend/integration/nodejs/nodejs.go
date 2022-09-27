// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/helpers"
	"namespacelabs.dev/foundation/schema"
)

func NewParser() api.IntegrationParser {
	return &helpers.SimpleJsonParser[*schema.NodejsIntegration]{
		SyntaxKind:     "namespace.so/from-nodejs",
		SyntaxShortcut: "nodejs",
	}
}
