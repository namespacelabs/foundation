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
	"namespacelabs.dev/foundation/runtime"
)

func serveLogs(s *SessionState, w http.ResponseWriter, r *http.Request, serverID string) {
	serveStream("server.logs", w, r, func(ctx context.Context, conn *websocket.Conn, wsWriter io.Writer) error {
		// XXX rather than obtaining the current one, it should be encoded in the request to logs.
		env, server, err := s.ResolveServer(ctx, serverID)
		if err != nil {
			return err
		}

		return runtime.For(env).StreamLogsTo(ctx, wsWriter, server, runtime.StreamLogsOpts{})
	})
}

func serveTaskOutput(s *SessionState, w http.ResponseWriter, r *http.Request, taskID, name string) {
	copyStream(fmt.Sprintf("task.output[%s]", name), w, r, func(ctx context.Context) (io.ReadCloser, error) {
		return s.TaskLogByName(taskID, name), nil
	})
}