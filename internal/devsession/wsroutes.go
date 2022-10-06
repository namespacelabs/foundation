// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devsession

import (
	"context"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RegisterEndpoints(s *Session, r *mux.Router) {
	r.HandleFunc("/ws/fn/command", fwd("command", func() io.ReadCloser {
		return s.CommandOutput()
	}))
	r.HandleFunc("/ws/fn/build", fwd("build", func() io.ReadCloser {
		return s.BuildOutput()
	}))
	r.HandleFunc("/ws/fn/build.json", fwd("build", func() io.ReadCloser {
		return s.BuildJSONOutput()
	}))
	r.HandleFunc("/ws/fn/stack", func(rw http.ResponseWriter, r *http.Request) {
		serveStack(s, rw, r)
	})
	r.HandleFunc("/ws/fn/server/{id}/logs", func(rw http.ResponseWriter, r *http.Request) {
		serveLogs(s, rw, r, mux.Vars(r)["id"])
	})
	r.HandleFunc("/ws/fn/server/{id}/terminal", func(rw http.ResponseWriter, r *http.Request) {
		serveTerminal(s, rw, r, mux.Vars(r)["id"])
	})
	r.HandleFunc("/ws/fn/task/{id}/output/{name}", func(rw http.ResponseWriter, r *http.Request) {
		v := mux.Vars(r)
		serveTaskOutput(s, rw, r, v["id"], v["name"])
	})
}

func fwd(kind string, f func() io.ReadCloser) func(http.ResponseWriter, *http.Request) {
	return func(rw http.ResponseWriter, r *http.Request) {
		copyStream(kind, rw, r, func(ctx context.Context) (io.ReadCloser, error) {
			output := f()
			if output == nil {
				return nil, status.Error(codes.OutOfRange, "no ongoing session")
			}

			return output, nil
		})
	}
}
