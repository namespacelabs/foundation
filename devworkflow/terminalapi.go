// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/console/termios"
	"namespacelabs.dev/foundation/runtime"
)

func serveTerminal(s *Session, w http.ResponseWriter, r *http.Request, serverID string) {
	serveStream("terminal", w, r, func(ctx context.Context, ws *websocket.Conn, w io.Writer) error {
		// XXX rather than obtaining the current one, it should be encoded in the request to logs.
		env, server, err := s.ResolveServer(r.Context(), serverID)
		if err != nil {
			return err
		}

		inr, inw := io.Pipe()

		resizeCh := make(chan termios.WinSize, 1)

		go func() {
			defer inr.Close()

			readerLoop(zerolog.Ctx(ctx).With().Logger(), ws, func(b []byte) error {
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

					zerolog.Ctx(ctx).Info().Uint32("width", w).Uint32("height", h).Msg("resizing terminal")
					resizeCh <- termios.WinSize{
						Width:  uint16(w),
						Height: uint16(h),
					}
				}

				return nil
			})
		}()

		// Returns when stdout is drained; which may happen when w fails to write, e.g. when ws is closed.
		return runtime.For(ctx, env).StartTerminal(ctx, server, runtime.TerminalIO{Stdin: inr, Stdout: w, Stderr: w, ResizeQueue: resizeCh}, "bash")
	})
}
