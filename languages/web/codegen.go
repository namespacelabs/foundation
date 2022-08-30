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
	"strings"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type generator struct{}

func (generator) Handle(ctx context.Context, env ops.Environment, _ *schema.SerializedInvocation, msg *OpGenHttpBackend) (*ops.HandleResult, error) {
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

func resolveBackend(wenv workspace.WorkspaceEnvironment, fragments []*schema.IngressFragment) genFunc {
	return func(ctx context.Context, loc workspace.Location, backend *OpGenHttpBackend_Backend) (*backendDefinition, error) {
		var matching []*schema.IngressFragment

		for _, fragment := range fragments {
			if fragment.GetOwner() != backend.EndpointOwner {
				continue
			}

			if backend.ServiceName != "" {
				if fragment.GetEndpoint().GetServiceName() != backend.ServiceName {
					continue
				}
			}

			if backend.IngressName != "" {
				if fragment.GetName() != backend.IngressName {
					continue
				}
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
				{"endpoint_owner", backend.EndpointOwner},
				{"service_name", backend.ServiceName},
				{"ingress_name", backend.IngressName},
				{"manager", backend.Manager},
			} {
				if r[1] != "" {
					matches = append(matches, fmt.Sprintf("%s=%q", r[0], r[1]))
				}
			}

			return nil, fnerrors.UserError(loc, "no ingress matches %s, perhaps you're missing `ingress: INTERNET_FACING`",
				strings.Join(matches, " "))
		}

		bd := &backendDefinition{}
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
