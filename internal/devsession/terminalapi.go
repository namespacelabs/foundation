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
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/framework/console/termios"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/runtime"
)

func serveTerminal(s sessionLike, w http.ResponseWriter, r *http.Request, serverID string) {
	serveStream("terminal", w, r, func(ctx context.Context, ws *websocket.Conn, w io.Writer) error {
		// XXX rather than obtaining the current one, it should be encoded in the request to logs.
		cluster, server, err := s.ResolveServer(r.Context(), serverID)
		if err != nil {
			return err
		}

		inr, inw := io.Pipe()

		resizeCh := make(chan termios.WinSize, 1)

		go func() {
			defer inr.Close()
			defer close(resizeCh)

			readerLoop(ctx, ws, func(b []byte) error {
				ti := &TerminalInput{}
				if err := protojson.Unmarshal(b, ti); err != nil {
					return err
				}

				if ti.Stdin != nil {
					if _, err := inw.Write(ti.Stdin); err != nil {
						return err
					}
				}

				if ti.Resize != nil {
					w := ti.Resize.Width
					h := ti.Resize.Height

					if w > 0xffff {
						w = 0xffff
					}

					if h > 0xffff {
						h = 0xffff
					}

					fmt.Fprintf(console.Debug(ctx), "(%s) resizing terminal %dx%d\n", r.RemoteAddr, w, h)

					resizeCh <- termios.WinSize{
						Width:  uint16(w),
						Height: uint16(h),
					}
				}

				return nil
			})
		}()

		// Returns when stdout is drained; which may happen when w fails to write, e.g. when ws is closed.
		return cluster.StartTerminal(ctx, server, runtime.TerminalIO{
			TTY:   true,
			Stdin: inr, Stdout: w, Stderr: w,
			ResizeQueue: resizeCh,
		}, "sh")
	})
}
