// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devsession

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

func serveLogs(s *Session, w http.ResponseWriter, r *http.Request, serverID string) {
	serveStream("server.logs", w, r, func(ctx context.Context, conn *websocket.Conn, wsWriter io.Writer) error {
		// XXX rather than obtaining the current one, it should be encoded in the request to logs.
		cluster, server, err := s.ResolveServer(ctx, serverID)
		if err != nil {
			return err
		}

		refs, err := cluster.ResolveContainers(ctx, server)
		if err != nil {
			return err
		}

		for _, ref := range refs {
			if ref.Kind == runtimepb.ContainerKind_PRIMARY {
				return cluster.Cluster().FetchLogsTo(ctx, wsWriter, ref, runtime.FetchLogsOpts{Follow: true})
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
