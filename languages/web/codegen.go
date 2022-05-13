// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type generator struct{}

func (generator) Handle(ctx context.Context, env ops.Environment, _ *schema.Definition, msg *OpGenHttpBackend) (*ops.HandleResult, error) {
	wenv, ok := env.(workspace.Packages)
	if !ok {
		return nil, errors.New("workspace.Packages required")
	}

	loc, err := wenv.Resolve(ctx, schema.PackageName(msg.Node.PackageName))
	if err != nil {
		return nil, err
	}

	fsys, err := generateBackendConf(ctx, loc, msg, generatePlaceholder(wenv), true)
	if err != nil {
		return nil, err
	}

	out := loc.Module.ReadWriteFS()
	return nil, fnfs.VisitFiles(ctx, fsys, func(path string, contents bytestream.ByteStream, de fs.DirEntry) error {
		info, err := de.Info()
		if err != nil {
			return err
		}

		return fnfs.WriteFileExtended(ctx, out, loc.Rel(path), info.Mode(), fnfs.WriteFileExtendedOpts{
			CompareContents: true,
			AnnounceWrite:   console.Stdout(ctx),
		}, func(w io.Writer) error {
			return bytestream.WriteTo(w, contents)
		})
	})
}

type genFunc func(context.Context, workspace.Location, *OpGenHttpBackend_Backend) (*backendDefinition, error)

func generatePlaceholder(loader workspace.Packages) genFunc {
	return func(ctx context.Context, loc workspace.Location, backend *OpGenHttpBackend_Backend) (*backendDefinition, error) {
		parsed, err := loader.LoadByName(ctx, schema.PackageName(backend.EndpointOwner))
		if err != nil {
			return nil, fnerrors.Wrapf(loc, err, "failed to load referenced endpoint %q", backend.EndpointOwner)
		}

		if parsed.Server == nil {
			return nil, fnerrors.UserError(loc, "%q must be a server", backend.EndpointOwner)
		}

		return nil, nil
	}
}

func resolveBackend(wenv workspace.WorkspaceEnvironment, endpoints []*schema.Endpoint) genFunc {
	return func(ctx context.Context, loc workspace.Location, backend *OpGenHttpBackend_Backend) (*backendDefinition, error) {
		for _, endpoint := range endpoints {
			if endpoint.EndpointOwner == backend.EndpointOwner && endpoint.ServiceName == backend.ServiceName {
				pkg, err := wenv.LoadByName(ctx, schema.PackageName(endpoint.EndpointOwner))
				if err != nil {
					return nil, fnerrors.Wrapf(loc, err, "failed to load dependency")
				}

				if pkg.Server == nil {
					return nil, fnerrors.InternalError("expected %q to be a server", endpoint.EndpointOwner)
				}

				plan, err := pkg.Parsed.EvalProvision(ctx, wenv, frontend.ProvisionInputs{ServerLocation: pkg.Location})
				if err != nil {
					return nil, fnerrors.InternalError("%s: failed to determine naming configuration: %w", pkg.Location.PackageName, err)
				}

				domains, err := runtime.ComputeDomains(wenv.Proto(), pkg.Server, plan.Naming, endpoint.AllocatedName)
				if err != nil {
					return nil, fnerrors.Wrapf(loc, err, "failed to compute domains")
				}

				bd := &backendDefinition{}
				for _, deferred := range domains {
					d := deferred.Domain
					if d.Managed == schema.Domain_LOCAL_MANAGED {
						bd.Managed = fmt.Sprintf("http://%s:%d", d.Fqdn, runtime.LocalIngressPort)
					} else if d.Managed == schema.Domain_CLOUD_MANAGED {
						bd.Managed = fmt.Sprintf("https://%s", d.Fqdn)
					} else {
						bd.Unmanaged = append(bd.Unmanaged, d.Fqdn)
					}
				}
				return bd, nil
			}
		}

		return nil, fnerrors.UserError(loc, "no such endpoint, endpoint_owner=%q service_name=%q",
			backend.EndpointOwner, backend.ServiceName)
	}
}

type backendDefinition struct {
	Managed   string   `json:"managed,omitempty"`
	Unmanaged []string `json:"unmanaged,omitempty"`
}

func generateBackendConf(ctx context.Context, loc workspace.Location, backend *OpGenHttpBackend, gen genFunc, placeholder bool) (*memfs.FS, error) {
	backends := map[string]*backendDefinition{}

	for _, b := range backend.Backend {
		backend, err := gen(ctx, loc, b)
		if err != nil {
			return nil, err
		}

		if backend == nil {
			backend = &backendDefinition{}
		}

		backends[b.InstanceName] = backend
	}

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
	fsys.Add("config/backends.fn.js", b.Bytes())
	return &fsys, nil
}
