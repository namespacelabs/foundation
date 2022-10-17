// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"strings"

	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/languages/nodejs/binary"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func generateWebBackend(ctx context.Context, _ *schema.SerializedInvocation, msg *OpGenHttpBackend) (*execution.HandleResult, error) {
	loader, err := execution.Get(ctx, pkggraph.PackageLoaderInjection)
	if err != nil {
		return nil, err
	}

	loc, err := loader.Resolve(ctx, schema.PackageName(msg.Node.PackageName))
	if err != nil {
		return nil, err
	}

	fsys, err := generateBackendConf(ctx, loc, msg, generatePlaceholder(loader), true)
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

type genFunc func(context.Context, pkggraph.Location, *OpGenHttpBackend_Backend) (*binary.BackendDefinition, error)

func generatePlaceholder(loader pkggraph.PackageLoader) genFunc {
	return func(ctx context.Context, loc pkggraph.Location, backend *OpGenHttpBackend_Backend) (*binary.BackendDefinition, error) {
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

func resolveBackend(fragments []*schema.IngressFragment) genFunc {
	return func(ctx context.Context, loc pkggraph.Location, backend *OpGenHttpBackend_Backend) (*binary.BackendDefinition, error) {
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

		bd := &binary.BackendDefinition{}
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

func generateBackendConf(ctx context.Context, loc pkggraph.Location, backend *OpGenHttpBackend, gen genFunc, placeholder bool) (*memfs.FS, error) {
	backends := map[string]*binary.BackendDefinition{}

	for _, b := range backend.Backend {
		backend, err := gen(ctx, loc, b)
		if err != nil {
			return nil, err
		}

		if backend == nil {
			backend = &binary.BackendDefinition{}
		}

		backends[b.InstanceName] = backend
	}

	return binary.GenerateBackendConfFromMap(ctx, backends, placeholder, "config/backends.fn.js")
}
