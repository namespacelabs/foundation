// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devsession

import (
	"context"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/runtime"
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
	RegisterSomeEndpoints(s, r)
	r.HandleFunc("/ws/fn/task/{id}/output/{name}", func(rw http.ResponseWriter, r *http.Request) {
		v := mux.Vars(r)
		serveTaskOutput(s, rw, r, v["id"], v["name"])
	})
}

type sessionLike interface {
	ResolveServer(ctx context.Context, serverID string) (runtime.ClusterNamespace, runtime.Deployable, error)
	NewClient(needsHistory bool) (ObserverLike, error)
	DeferRequest(req *DevWorkflowRequest)
}

type ObserverLike interface {
	Events() chan *Update
	Close()
}

func RegisterSomeEndpoints(s sessionLike, r *mux.Router) {
	r.HandleFunc("/ws/fn/stack", func(rw http.ResponseWriter, r *http.Request) {
		serveStack(s, rw, r)
	})
	r.HandleFunc("/ws/fn/server/{id}/logs", func(rw http.ResponseWriter, r *http.Request) {
		serveLogs(s, rw, r, mux.Vars(r)["id"])
	})
	r.HandleFunc("/ws/fn/server/{id}/terminal", func(rw http.ResponseWriter, r *http.Request) {
		serveTerminal(s, rw, r, mux.Vars(r)["id"])
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
