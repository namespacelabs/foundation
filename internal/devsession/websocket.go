// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devsession

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/std/tasks"
)

func isCloseError(err error) bool {
	// We check the error message, because poll.ErrNetClosing is not exported :(
	// https://cs.opensource.google/go/go/+/master:src/internal/poll/fd.go;l=31
	if strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}

	_, ok := err.(*websocket.CloseError)
	return ok
}

func readerLoop(ctx context.Context, ws *websocket.Conn, f func([]byte) error) {
	ws.SetReadLimit(4096)

	for {
		t, msg, err := ws.ReadMessage()

		if err != nil {
			// Closing the websocket may happen for various reasons and it is not an exception.
			if !isCloseError(err) {
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
	upgrader := newWebsocketUpgrader(r)

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
		if !errors.Is(err, context.Canceled) {
			fmt.Fprintf(writer, "failed: %v\n", err)
			fmt.Fprintf(console.Errors(r.Context()), "(%s) websocket: failed: %v\n", r.RemoteAddr, err)
		}
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

func newWebsocketUpgrader(r *http.Request) *websocket.Upgrader {
	upgrader := websocket.Upgrader{
		ReadBufferSize:    64 * 1024,
		WriteBufferSize:   64 * 1024,
		EnableCompression: true,
	}

	// Allowing all requests from "localhost".
	// This is needed for the case of running inside a Gitpod instance due to the way Gitpod does
	// port forwarding:
	//   r.Host is "localhost:<port>" in this case.
	if isLocalhost(r.Host) {
		upgrader.CheckOrigin = func(rr *http.Request) bool { return true }
	}
	return &upgrader
}

func isLocalhost(host string) bool {
	if host == "localhost" {
		return true
	}
	if strings.HasPrefix(host, "localhost:") {
		return true
	}
	return false
}
