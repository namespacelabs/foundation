// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package binary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// Generates a "backends.fn.js" with ingress addresses for required backends.
func generateBackendsConfig(ctx context.Context, loc pkggraph.Location, backends []*schema.NodejsBuild_Backend, ingressFragments compute.Computable[[]*schema.IngressFragment], opts *BackendsOpts) ([]byte, error) {
	var fragments []*schema.IngressFragment
	if ingressFragments != nil {
		var err error
		fragments, err = compute.GetValue(ctx, ingressFragments)
		if err != nil {
			return nil, fnerrors.InternalError("failed to build a nodejs app while waiting on ingress computation: %w", err)
		}
	}

	backendsMap := map[string]*BackendDefinition{}

	for _, backend := range backends {
		if opts != nil && opts.Placeholder {
			backendsMap[backend.Name] = &BackendDefinition{
				Managed: "placeholder",
			}
		} else {
			backendDef, err := resolveBackend(loc, backend, fragments, opts)
			if err != nil {
				return nil, err
			}

			backendsMap[backend.Name] = backendDef
		}
	}

	return GenerateBackendConfFromMap(ctx, backendsMap, opts)
}

type BackendDefinition struct {
	Managed   string   `json:"managed,omitempty"`
	Unmanaged []string `json:"unmanaged,omitempty"`
}

type BackendsOpts struct {
	// If true, generate a placeholder file with a comment explaining that the
	// actual values are resolved at build time.
	Placeholder bool
	// If true, the URLs in the generated file will be in-cluster addresses rather than from ingress.
	UseInClusterAddresses bool
}

func GenerateBackendConfFromMap(ctx context.Context, backends map[string]*BackendDefinition, opts *BackendsOpts) ([]byte, error) {
	var b bytes.Buffer

	fmt.Fprintln(&b, "// This is an automatically generated file.")
	if opts != nil && opts.Placeholder {
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

	return b.Bytes(), nil
}

func resolveBackend(loc pkggraph.Location, backend *schema.NodejsBuild_Backend, fragments []*schema.IngressFragment, opts *BackendsOpts) (*BackendDefinition, error) {
	var matching []*schema.IngressFragment

	for _, fragment := range fragments {
		if fragment.GetOwner() != backend.Service.PackageName {
			continue
		}

		if fragment.GetEndpoint().GetServiceName() != backend.Service.Name {
			continue
		}

		if backend.Manager != "" {
			if fragment.GetManager() != backend.Manager {
				continue
			}
		}

		matching = append(matching, fragment)
	}

	if len(matching) == 0 {
		var matches []string
		for _, r := range [][2]string{
			{"endpoint_owner", backend.Service.PackageName},
			{"service_name", backend.Service.Name},
			{"manager", backend.Manager},
		} {
			if r[1] != "" {
				matches = append(matches, fmt.Sprintf("%s=%q", r[0], r[1]))
			}
		}

		return nil, fnerrors.NewWithLocation(loc, "no ingress matches %s, perhaps you're missing `ingress: true`",
			strings.Join(matches, " "))
	}

	bd := &BackendDefinition{}
	for _, fragment := range matching {
		if opts != nil && opts.UseInClusterAddresses {
			bd.Managed = fmt.Sprintf("http://%s:%d", fragment.Endpoint.AllocatedName, fragment.Endpoint.ExportedPort)
		} else {
			d := fragment.Domain
			if d.Managed == schema.Domain_LOCAL_MANAGED {
				bd.Managed = fmt.Sprintf("http://%s:%d", d.Fqdn, runtime.LocalIngressPort)
			} else if d.TlsFrontend {
				bd.Managed = fmt.Sprintf("https://%s", d.Fqdn)
			} else {
				bd.Managed = fmt.Sprintf("http://%s", d.Fqdn)
			}
		}
	}
	return bd, nil
}
