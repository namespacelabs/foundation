// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cluster

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/chisel/share/cnet"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/std/tasks"
)

func TestInlineSshExitStatus(t *testing.T) {
	for _, tt := range []struct {
		name    string
		status  uint32
		signal  string
		shell   bool
		missing bool
		want    int
	}{
		{name: "success"},
		{name: "ordinary failure", status: 23, want: 23},
		{name: "SIGTERM status", status: 143, want: 143},
		{name: "SIGKILL status", status: 137, want: 137},
		{name: "SIGTERM signal", signal: "TERM", want: 143},
		{name: "SIGKILL signal", signal: "KILL", want: 137},
		{name: "shell SIGTERM status", shell: true, status: 143, want: 143},
		{name: "shell SIGTERM signal", shell: true, signal: "TERM", want: 143},
		{name: "missing status", missing: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, key, err := ed25519.GenerateKey(rand.Reader)
			require.NoError(t, err)
			signer, err := ssh.NewSignerFromKey(key)
			require.NoError(t, err)
			privateKey, err := ssh.MarshalPrivateKey(key, "")
			require.NoError(t, err)

			config := &ssh.ServerConfig{NoClientAuth: true}
			config.AddHostKey(signer)
			done := make(chan struct{})
			gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(done)
				ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if !assert.NoError(t, err) {
					return
				}
				conn := cnet.NewWebSocketConn(ws)
				defer conn.Close()
				conn.SetDeadline(time.Now().Add(10 * time.Second))
				server, channels, requests, err := ssh.NewServerConn(conn, config)
				if !assert.NoError(t, err) {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for ch := range channels {
					channel, requests, err := ch.Accept()
					if !assert.NoError(t, err) {
						return
					}
					defer channel.Close()
					for req := range requests {
						if req.Type != "exec" && req.Type != "shell" {
							req.Reply(false, nil)
							continue
						}
						req.Reply(true, nil)
						if tt.signal != "" {
							_, err = channel.SendRequest("exit-signal", false, ssh.Marshal(struct {
								Signal     string
								CoreDumped bool
								Error      string
								Lang       string
							}{Signal: tt.signal}))
						} else if !tt.missing {
							_, err = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{tt.status}))
						}
						assert.NoError(t, err)
						assert.NoError(t, channel.Close())
						// Keep the transport open until the client consumes the channel close.
						break
					}
				}
			}))
			defer gateway.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctx = tasks.WithSink(ctx, tasks.NullSink())
			ctx = fnapi.WithToken(ctx, &auth.Token{ReIssue: func(context.Context, *auth.Token, time.Duration) (string, error) {
				return "test-token", nil
			}})
			instance := &api.KubernetesCluster{
				SshPrivateKey: pem.EncodeToMemory(privateKey),
				ServiceState: []*api.Cluster_ServiceState{{
					Name: "ssh", Status: "READY", Endpoint: "ws" + strings.TrimPrefix(gateway.URL, "http"),
				}},
			}
			args := []string{"test-command"}
			if tt.shell {
				args = nil
			}
			err = InlineSsh(ctx, instance, InlineSshOpts{DisablePty: true}, args)
			select {
			case <-done:
			case <-ctx.Done():
				t.Fatal("SSH test server did not finish")
			}
			if tt.missing {
				var missing *ssh.ExitMissingError
				require.ErrorAs(t, err, &missing)
				_, ok := err.(fnerrors.ExitError)
				require.False(t, ok, "protocol failures must not be treated as command exit statuses")
			} else if tt.want == 0 {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				exit, ok := err.(fnerrors.ExitError)
				require.True(t, ok, "CLI would turn %T (%v) into exit code 1", err, err)
				require.Equal(t, tt.want, exit.ExitCode())
			}
		})
	}
}
