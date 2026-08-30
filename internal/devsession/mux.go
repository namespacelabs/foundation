// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devsession

import (
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/tasks"
)

// serveFS returns a Computable[*mux.Router]. If `spa` is true (i.e. single page app),
// and an index.html is present, it is served on all paths (except the ones for which)
// real files exist.
func serveFS(image compute.Computable[oci.Image], pathPrefix string, spa bool) compute.Computable[*mux.Router] {
	return &fsServing{image: image, spa: spa, pathPrefix: pathPrefix}
}

type fsServing struct {
	image      compute.Computable[oci.Image]
	spa        bool
	pathPrefix string

	compute.LocalScoped[*mux.Router]
}

func (m *fsServing) Action() *tasks.ActionEvent { return tasks.Action("web.mux") }
func (m *fsServing) Inputs() *compute.In {
	return compute.Inputs().Computable("image", m.image).Bool("spa", m.spa)
}
func (m *fsServing) Compute(ctx context.Context, deps compute.Resolved) (*mux.Router, error) {
	image, _ := compute.GetDep(deps, m.image, "image")

	return muxFromFS(ctx, oci.ImageAsFS(image.Value), image.Digest, image.Completed, m.spa, m.pathPrefix)
}

func muxFromFS(ctx context.Context, fsys fs.FS, d schema.Digest, ts time.Time, spa bool, pathPrefix string) (*mux.Router, error) {
	r := mux.NewRouter()

	if err := fnfs.VisitFiles(ctx, fsys, func(path string, blob bytestream.ByteStream, _ fs.DirEntry) error {
		var route *mux.Route

		path = path[len(pathPrefix):]

		if path == "index.html" {
			if spa {
				route = r.PathPrefix("/")
			} else {
				route = r.Path("/")
			}
		} else {
			route = r.Path("/" + path)
		}

		contents, err := bytestream.ReadAll(blob)
		if err != nil {
			return err
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
