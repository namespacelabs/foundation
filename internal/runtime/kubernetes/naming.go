// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"path/filepath"
	"regexp"
	"strings"

	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/schema"
)

var (
	validChars = regexp.MustCompile("[a-z0-9]+")
)

// We use namespaces to isolate deployments per workspace and environment.
// Using the path base plus a digest provides short, memorable names and avoids collision.
// TODO add knob to allow namespace overwrites if the need arises.
func ModuleNamespace(ws *schema.Workspace, env *schema.Environment) string {
	parts := []string{strings.ToLower(env.Name)}
	parts = append(parts, validChars.FindAllString(filepath.Base(ws.ModuleName), -1)...)

	id := naming.StableIDN(ws.ModuleName, 5)

	parts = append(parts, id)
	return strings.Join(parts, "-")
}
