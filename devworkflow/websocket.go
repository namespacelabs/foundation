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
	"github.com/rs/zerolog"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func readerLoop(l zerolog.Logger, ws *websocket.Conn, f func([]byte) error) {
	ws.SetReadLimit(4096)

	l = l.With().Str("remoteAddr", ws.RemoteAddr().String()).Logger()

	for {
		t, msg, err := ws.ReadMessage()

		if err != nil {
			// Not reporting CloseError's.
			// Closing the websocket may happen for various reasons and it is not an exception.
			if _, ok := err.(*websocket.CloseError); !ok {
				l.Err(err).Msg("WebSocket.ReadMessage failed, bailing out")
			}
			break
		}

		if (t == websocket.TextMessage || t == websocket.BinaryMessage) && f != nil {
			if err := f(msg); err != nil {
				l.Err(err).Str("msg", string(msg)).Msg("WebSocket.ReadMessage failed on user, bailing out")
				break
			}
		} else {
			l.Warn().
				Int("type", t).
				Int("len", len(msg)).
				Msg("unhandled websocket message")
		}
	}
}

func writeJSONLoop(ctx context.Context, ws *websocket.Conn, ch chan *Update) {
	defer ws.Close() // On error, close the ws so the reader loop also exits.

	l := zerolog.Ctx(ctx).With().Str("remoteAddr", ws.RemoteAddr().String()).Logger()

	for {
		select {
		case <-ctx.Done():
			l.Debug().Msg("done")
			return

		case newUpdate := <-ch:
			data, err := tasks.TryProtoAsJson(nil, newUpdate, false)
			if err != nil {
				l.Err(err).Msg("failed to serialize")
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, data); err != nil {
				l.Err(err).Msg("failed to write")
				return
			}

			l.Trace().Int("len", len(data)).Msg("pushed JSON")
		}
	}
}

func serveStream(kind string, w http.ResponseWriter, r *http.Request, handler func(context.Context, *websocket.Conn, io.Writer) error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:    64 * 1024,
		WriteBufferSize:   64 * 1024,
		EnableCompression: true,
	}

	l := zerolog.Ctx(r.Context()).With().Str("remoteAddr", r.RemoteAddr).Str("websocket", kind).Logger()

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			l.Err(err).Msg("websocket upgrade failed")
		}
		return
	}

	_ = ws.SetCompressionLevel(6)

	writer := writeStream{ws}

	// Make sure that Logs() is cancelled if the websocket is closed.
	ctxWithCancel, cancel := context.WithCancel(r.Context())
	defer cancel()

	defer ws.Close()

	if err := handler(l.WithContext(ctxWithCancel), ws, writer); err != nil {
		fmt.Fprintf(writer, "failed: %v\n", err)
		l.Err(err).Msg("websocket failed")
	}
}

func copyStream(kind string, w http.ResponseWriter, r *http.Request, f func(context.Context) (io.ReadCloser, error)) {
	serveStream(kind, w, r, func(ctx context.Context, ws *websocket.Conn, writer io.Writer) error {
		stream, err := f(ctx)
		if err != nil {
			return err
		}
		if stream == nil {
			return status.Error(codes.NotFound, "no such stream")
		}

		defer ws.Close()
		defer stream.Close()

		go func() {
			if _, err := io.Copy(writer, stream); err != nil {
				zerolog.Ctx(ctx).Err(err).Msg("websocket stream write failed")
			}

			// Tell the reader to bail out.
			if err := ws.Close(); err != nil {
				zerolog.Ctx(ctx).Err(err).Msg("websocket close failed")
			}
		}()

		readerLoop(zerolog.Ctx(ctx).With().Logger(), ws, nil)
		return nil
	})
}

type writeStream struct{ ws *websocket.Conn }

func (w writeStream) Write(p []byte) (int, error) {
	return len(p), w.ws.WriteMessage(websocket.BinaryMessage, p)
}
