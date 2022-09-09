// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

func serveLogs(s *Session, w http.ResponseWriter, r *http.Request, serverID string) {
	serveStream("server.logs", w, r, func(ctx context.Context, conn *websocket.Conn, wsWriter io.Writer) error {
		// XXX rather than obtaining the current one, it should be encoded in the request to logs.
		env, server, err := s.ResolveServer(ctx, serverID)
		if err != nil {
			return err
		}

		rt := runtime.ClusterFor(ctx, env)
		refs, err := rt.ResolveContainers(ctx, server)
		if err != nil {
			return err
		}

		for _, ref := range refs {
			if ref.Kind == schema.ContainerKind_PRIMARY {
				return runtime.ClusterFor(ctx, env).FetchLogsTo(ctx, wsWriter, ref, runtime.FetchLogsOpts{Follow: true})
			}
		}

		return fnerrors.InvocationError("server has no identifiable primary container")
	})
}

func serveTaskOutput(s *Session, w http.ResponseWriter, r *http.Request, taskID, name string) {
	copyStream(fmt.Sprintf("task.output[%s]", name), w, r, func(ctx context.Context) (io.ReadCloser, error) {
		return s.TaskLogByName(taskID, name), nil
	})
}
