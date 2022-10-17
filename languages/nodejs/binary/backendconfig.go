// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package binary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

const (
	backendsConfigFn = "src/config/backends.ns.js"
)

// Generates a "backends.fn.js" with ingress addresses for required backends.
func generateBackendsConfig(ctx context.Context, loc pkggraph.Location, backends []*schema.NodejsIntegration_Backend, ingressFragments compute.Computable[[]*schema.IngressFragment]) (fs.FS, error) {
	fragments, err := compute.GetValue(ctx, ingressFragments)
	if err != nil {
		return nil, fnerrors.InternalError("failed to build a nodejs app while waiting on ingress computation: %w", err)
	}

	backendsMap := map[string]*BackendDefinition{}

	for _, backend := range backends {
		backendDef, err := resolveBackend(loc, backend.Service, fragments)
		if err != nil {
			return nil, err
		}

		backendsMap[backend.Name] = backendDef
	}

	return GenerateBackendConfFromMap(ctx, backendsMap, false /* placeholder */, backendsConfigFn)
}

type BackendDefinition struct {
	Managed   string   `json:"managed,omitempty"`
	Unmanaged []string `json:"unmanaged,omitempty"`
}

func GenerateBackendConfFromMap(ctx context.Context, backends map[string]*BackendDefinition, placeholder bool, filename string) (*memfs.FS, error) {
	var b bytes.Buffer

	fmt.Fprintln(&b, "// This is an automatically generated file.")
	if placeholder {
		fmt.Fprintln(&b, "//")
		fmt.Fprintln(&b, "// This placeholder file exists as a convenience. The actual values are")
		fmt.Fprintln(&b, "// resolved at build time, when the build is bound to an environment")
		fmt.Fprintln(&b, "// and the server dependencies can be introspected.")
		fmt.Fprintln(&b, "//")
		fmt.Fprintln(&b, "// Each backend will have a list of URLs, separated by foundation-managed")
		fmt.Fprintln(&b, "// domains, and user-specified. E.g.")
		fmt.Fprintln(&b, "//")
		fmt.Fprintln(&b, "//   export const Backends = {")
		fmt.Fprintln(&b, "//     apiBackend: {")
		fmt.Fprintln(&b, "//       managed: 'foobar.prod.org.nscloud.dev'")
		fmt.Fprintln(&b, "//       unmanaged: ['foobar.myorg.com']")
		fmt.Fprintln(&b, "//     }")
		fmt.Fprintln(&b, "//   }")
		fmt.Fprintln(&b, "//")
		fmt.Fprintln(&b, "//")
	}
	fmt.Fprintln(&b)
	fmt.Fprint(&b, "export const Backends = ")

	enc := json.NewEncoder(&b)
	enc.SetIndent("", "  ")
	if err := enc.Encode(backends); err != nil {
		return nil, err
	}

	var fsys memfs.FS
	fsys.Add(filename, b.Bytes())
	return &fsys, nil
}

func resolveBackend(loc pkggraph.Location, serviceRef *schema.PackageRef, fragments []*schema.IngressFragment) (*BackendDefinition, error) {
	var matching []*schema.IngressFragment

	for _, fragment := range fragments {
		if fragment.GetOwner() != serviceRef.PackageName {
			continue
		}

		if fragment.GetEndpoint().GetServiceName() != serviceRef.Name {
			continue
		}

		matching = append(matching, fragment)
	}

	if len(matching) == 0 {
		var matches []string
		for _, r := range [][2]string{
			{"endpoint_owner", serviceRef.PackageName},
			{"service_name", serviceRef.Name},
		} {
			if r[1] != "" {
				matches = append(matches, fmt.Sprintf("%s=%q", r[0], r[1]))
			}
		}

		return nil, fnerrors.UserError(loc, "no ingress matches %s, perhaps you're missing `ingress: INTERNET_FACING`",
			strings.Join(matches, " "))
	}

	bd := &BackendDefinition{}
	for _, fragment := range matching {
		d := fragment.Domain
		if d.Managed == schema.Domain_LOCAL_MANAGED {
			bd.Managed = fmt.Sprintf("http://%s:%d", d.Fqdn, runtime.LocalIngressPort)
		} else if d.Managed == schema.Domain_CLOUD_MANAGED || d.Managed == schema.Domain_CLOUD_TERMINATION {
			if d.TlsFrontend {
				bd.Managed = fmt.Sprintf("https://%s", d.Fqdn)
			} else {
				bd.Managed = fmt.Sprintf("http://%s", d.Fqdn)
			}
		} else {
			bd.Unmanaged = append(bd.Unmanaged, d.Fqdn)
		}
	}
	return bd, nil
}
