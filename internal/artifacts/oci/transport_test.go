// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	registryserver "github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
	"namespacelabs.dev/foundation/internal/build/registry"
)

func TestSSHRegistryReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	handler := registryserver.New(registryserver.Logger(log.New(io.Discard, "", 0)))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Force each request to open an SSH channel rather than reuse HTTP connections.
		w.Header().Set("Connection", "close")
		handler.ServeHTTP(w, r)
	}))
	defer server.Close()
	address, dropNext := serveSSHForwarder(t, server.Listener.Addr().String())
	ref, err := name.NewTag("registry.test:5000/test/image:latest", name.Insecure)
	if !assert.NoError(t, err) {
		return
	}
	opts, err := RemoteOptsWithAuth(ctx, RegistryAccess{
		PublicImage: true,
		Transport: &registry.RegistryTransport{Ssh: &registry.RegistryTransport_SSH{
			User:       "test",
			SshAddr:    address,
			RemoteAddr: server.Listener.Addr().String(),
		}},
	}, true)
	if !assert.NoError(t, err) {
		return
	}
	if !assert.NoError(t, remote.Write(ref, empty.Image, opts...)) {
		return
	}
	expected, err := empty.Image.Digest()
	if !assert.NoError(t, err) {
		return
	}

	dropNext.Store(true)
	if !assert.NoError(t, remote.Write(ref, empty.Image, opts...)) {
		return
	}
	dropNext.Store(true)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			image, err := remote.Get(ref, opts...)
			if assert.NoError(t, err) {
				assert.Equal(t, expected, image.Digest)
			}
		})
	}
	wg.Wait()
}

func serveSSHForwarder(t *testing.T, target string) (string, *atomic.Bool) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	signer, err := ssh.NewSignerFromKey(key)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	config := &ssh.ServerConfig{NoClientAuth: true}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	dropNext := &atomic.Bool{}
	var wg sync.WaitGroup
	t.Cleanup(wg.Wait)
	t.Cleanup(func() { listener.Close() })
	wg.Go(func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { conn.Close() })
			wg.Go(func() {
				server, channels, requests, err := ssh.NewServerConn(conn, config)
				if err != nil {
					conn.Close()
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for channel := range channels {
					if dropNext.Swap(false) {
						conn.Close()
						return
					}
					upstream, err := net.DialTimeout("tcp", target, time.Second)
					if err != nil {
						channel.Reject(ssh.ConnectionFailed, err.Error())
						continue
					}
					stream, requests, err := channel.Accept()
					if err != nil {
						upstream.Close()
						continue
					}
					go ssh.DiscardRequests(requests)
					wg.Go(func() {
						defer upstream.Close()
						defer stream.Close()
						go func() {
							io.Copy(upstream, stream)
							upstream.(*net.TCPConn).CloseWrite()
						}()
						io.Copy(stream, upstream)
					})
				}
			})
		}
	})
	return listener.Addr().String(), dropNext
}
