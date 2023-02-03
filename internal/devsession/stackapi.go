// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devsession

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"namespacelabs.dev/foundation/internal/console"
)

func serveStack(s sessionLike, w http.ResponseWriter, r *http.Request) {
	upgrader := newWebsocketUpgrader(r)

	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			fmt.Fprintf(console.Errors(r.Context()), "(%s) websocket: failed to handshake: %v\n", r.RemoteAddr, err)
		}
		w.WriteHeader(500)
		fmt.Fprintf(w, "%v", err)
		return
	}

	defer ws.Close()

	_ = ws.SetCompressionLevel(6)

	fmt.Fprintf(console.Debug(r.Context()), "(%s) websocket: connected\n", r.RemoteAddr)

	ch, err := s.NewClient(true)
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprintf(w, "%v", err)
		return
	}

	defer ch.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel() // Important that this is the first deferred call, before closing the channel.

	go writeJSONLoop(ctx, ws, ch.Events())

	readerLoop(r.Context(), ws, func(msg []byte) error {
		m := &DevWorkflowRequest{}
		if err := protojson.Unmarshal(msg, m); err != nil {
			return err
		}

		// Push it to be processed.
		s.DeferRequest(m)
		return nil
	})
}
