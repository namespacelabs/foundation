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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func readerLoop(ctx context.Context, ws *websocket.Conn, f func([]byte) error) {
	ws.SetReadLimit(4096)

	for {
		t, msg, err := ws.ReadMessage()

		if err != nil {
			// Not reporting CloseError's.
			// Closing the websocket may happen for various reasons and it is not an exception.
			if _, ok := err.(*websocket.CloseError); !ok {
				fmt.Fprintf(console.Errors(ctx), "(%s) websocket: read message failed: %v\n", ws.RemoteAddr(), err)
			}
			break
		}

		if (t == websocket.TextMessage || t == websocket.BinaryMessage) && f != nil {
			if err := f(msg); err != nil {
				fmt.Fprintf(console.Errors(ctx), "(%s) websocket: message handler failed: %v\n", ws.RemoteAddr(), err)
				break
			}
		} else {
			fmt.Fprintf(console.Errors(ctx), "(%s) websocket: unhandled message type: %d\n", ws.RemoteAddr(), t)
		}
	}
}

func writeJSONLoop(ctx context.Context, ws *websocket.Conn, ch chan *Update) {
	defer ws.Close() // On error, close the ws so the reader loop also exits.

	for {
		select {
		case <-ctx.Done():
			return

		case newUpdate := <-ch:
			data, err := tasks.TryProtoAsJson(nil, newUpdate, false)
			if err != nil {
				fmt.Fprintf(console.Errors(ctx), "(%s) websocket: failed to serialize: %v\n", ws.RemoteAddr(), err)
				return
			}
			if err := ws.WriteMessage(websocket.BinaryMessage, data); err != nil {
				fmt.Fprintf(console.Errors(ctx), "(%s) websocket: failed to write: %v\n", ws.RemoteAddr(), err)
				return
			}
		}
	}
}

func serveStream(kind string, w http.ResponseWriter, r *http.Request, handler func(context.Context, *websocket.Conn, io.Writer) error) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:    64 * 1024,
		WriteBufferSize:   64 * 1024,
		EnableCompression: true,
	}

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			fmt.Fprintf(console.Errors(r.Context()), "(%s) websocket: upgrade failed: %v\n", r.RemoteAddr, err)
		}
		return
	}

	_ = ws.SetCompressionLevel(6)

	writer := writeStream{ws}

	// Make sure that Logs() is cancelled if the websocket is closed.
	ctxWithCancel, cancel := context.WithCancel(r.Context())
	defer cancel()

	defer ws.Close()

	if err := handler(ctxWithCancel, ws, writer); err != nil {
		fmt.Fprintf(writer, "failed: %v\n", err)
		fmt.Fprintf(console.Errors(r.Context()), "(%s) websocket: failed: %v\n", r.RemoteAddr, err)
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
				fmt.Fprintf(console.Errors(ctx), "(%s) websocket: stream write failed: %v\n", ws.RemoteAddr(), err)
			}

			// Tell the reader to bail out.
			if err := ws.Close(); err != nil {
				fmt.Fprintf(console.Errors(ctx), "(%s) websocket: stream close failed: %v\n", ws.RemoteAddr(), err)
			}
		}()

		readerLoop(ctx, ws, nil)
		return nil
	})
}

type writeStream struct{ ws *websocket.Conn }

func (w writeStream) Write(p []byte) (int, error) {
	return len(p), w.ws.WriteMessage(websocket.BinaryMessage, p)
}
