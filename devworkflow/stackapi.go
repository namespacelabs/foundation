// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
	"google.golang.org/protobuf/encoding/protojson"
)

func serveStack(s *Session, w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:    64 * 1024,
		WriteBufferSize:   64 * 1024,
		EnableCompression: true,
	}

	l := zerolog.Ctx(r.Context()).With().Str("remoteAddr", r.RemoteAddr).Logger()

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			l.Err(err).Msg("failed to handshake websocket")
		}
		return
	}

	_ = ws.SetCompressionLevel(6)

	l.Debug().Str("url", r.URL.String()).Msg("connected")

	ch, cancel := s.NewClient()
	defer cancel()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel() // Important that this is the first deferred call, before closing the channel.

	go writeJSONLoop(ctx, ws, ch)

	readerLoop(zerolog.Ctx(r.Context()).With().Logger(), ws, func(msg []byte) error {
		m := &DevWorkflowRequest{}
		if err := protojson.Unmarshal(msg, m); err != nil {
			return err
		}

		// Push it to be processed.
		s.Ch <- m
		return nil
	})
}
