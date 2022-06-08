// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"namespacelabs.dev/foundation/runtime/kubernetes/controller"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
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
func moduleNamespace(ws *schema.Workspace, env *schema.Environment) string {
	parts := []string{strings.ToLower(env.Name)}
	parts = append(parts, validChars.FindAllString(filepath.Base(ws.ModuleName), -1)...)

	h := sha256.New()
	h.Write([]byte(ws.ModuleName)) // Write to a sha256 hash never fails.
	digest := h.Sum(nil)

	// A SHA256 is 32 bytes long, we're guarantee to always have at least 5 characters.
	parts = append(parts, base32encoding.EncodeToString(digest)[:5])
	return strings.Join(parts, "-")
}

func shortPackageName(pkg schema.PackageName) string {
	parts := strings.Split(pkg.String(), "/")
	return fmt.Sprintf("%s-%s", parts[len(parts)-1], packageId(pkg))
}

func packageId(pkg schema.PackageName) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s", pkg) // Write to a sha256 hash never fails.
	digest := h.Sum(nil)
	return base32encoding.EncodeToString(digest[:8])
}

func serverNamespace(r K8sRuntime, srv *schema.Server) string {
	if controller.IsController(schema.PackageName(srv.PackageName)) {
		return kubedef.AdminNamespace
	}

	return r.moduleNamespace
}
