// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"fmt"
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

func VolumeName(v *schema.Volume) string {
	if v.Inline {
		// generate a k8s-compliant name.
		parts := validChars.FindAllString(filepath.Base(v.Name), -1)
		parts = append(parts, naming.StableIDN(v.Name, 5))
		return fmt.Sprintf("v-inline-%s", strings.Join(parts, "-"))
	}

	// TODO should we really change the user-provided name?
	return fmt.Sprintf("v-%s", v.Name)
}
