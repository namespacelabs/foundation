// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package docker

import (
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/entity"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/helpers"
	"namespacelabs.dev/foundation/schema"
)

func NewParser() entity.EntityParser {
	return &helpers.SimpleJsonParser[*schema.DockerIntegration]{
		SyntaxUrl:      "namespace.so/from-dockerfile",
		SyntaxShortcut: "docker",
	}
}
