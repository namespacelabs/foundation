// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"encoding/base32"
	"path/filepath"
	"regexp"
	"strings"

	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/schema"
)

const (
	lowerCaseEncodeBase32 = "0123456789abcdefghijklmnopqrstuv"
)

var (
	validChars     = regexp.MustCompile("[a-z0-9]+")
	base32encoding = base32.NewEncoding(lowerCaseEncodeBase32).WithPadding(base32.NoPadding)
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
