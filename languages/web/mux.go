// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package web

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/tarfs"
	"namespacelabs.dev/foundation/internal/fntypes"
	"namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/tasks"
)

// ServeFS returns a Computable[*mux.Router]. If `spa` is true (i.e. single page app),
// and an index.html is present, it is served on all paths (except the ones for which)
// real files exist.
func ServeFS(image compute.Computable[oci.Image], spa bool) compute.Computable[*mux.Router] {
	return &serveFS{image: image, spa: spa}
}

type serveFS struct {
	image compute.Computable[oci.Image]
	spa   bool

	compute.LocalScoped[*mux.Router]
}

func (m *serveFS) Action() *tasks.ActionEvent { return tasks.Action("web.mux") }
func (m *serveFS) Inputs() *compute.In {
	return compute.Inputs().Computable("image", m.image).Bool("spa", m.spa)
}
func (m *serveFS) Compute(ctx context.Context, deps compute.Resolved) (*mux.Router, error) {
	image, _ := compute.GetDep(deps, m.image, "image")

	fsys := tarfs.FS{
		TarStream: func() (io.ReadCloser, error) {
			return mutate.Extract(image.Value), nil
		},
	}

	return MuxFromFS(ctx, fsys, image.Digest, image.Timestamp, m.spa)
}

func MuxFromFS(ctx context.Context, fsys fs.FS, d fntypes.Digest, ts time.Time, spa bool) (*mux.Router, error) {
	r := mux.NewRouter()

	if err := fnfs.VisitFiles(ctx, fsys, func(path string, contents []byte, _ fs.DirEntry) error {
		var route *mux.Route

		if path == "index.html" {
			if spa {
				route = r.PathPrefix("/")
			} else {
				route = r.Path("/")
			}
		} else {
			route = r.Path("/" + path)
		}

		route.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if d.IsSet() {
				req.Header.Add("ETag", d.String())
			}
			http.ServeContent(rw, req, path, ts, bytes.NewReader(contents))
		})
		return nil
	}); err != nil {
		return nil, err
	}

	return r, nil
}