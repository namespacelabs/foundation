// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"
	"net"
	"net/http"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/ssh"
	"namespacelabs.dev/foundation/internal/tcache"
)

var (
	sshHttpTransports = tcache.NewCache[*http.Transport]()
)

func parseTransport(ctx context.Context, t *registry.RegistryTransport) ([]remote.Option, error) {
	if t == nil {
		return nil, nil
	}

	switch {
	case t.Ssh != nil:
		remoteAddr := t.Ssh.RemoteAddr
		if remoteAddr == "" {
			return nil, fnerrors.New("transport.ssh: missing remote address")
		}

		deferred, err := ssh.Establish(ctx, ssh.Endpoint{
			User:           t.Ssh.User,
			PrivateKeyPath: t.Ssh.PrivateKeyPath,
			Address:        t.Ssh.SshAddr,
		})
		if err != nil {
			return nil, err
		}

		transport, err := sshHttpTransports.Compute(deferred.CacheKey, func() (*http.Transport, error) {
			conn, err := deferred.Dial()
			if err != nil {
				return nil, err
			}

			// XXX conn.Close

			return &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return conn.Dial("tcp", remoteAddr)
				},
			}, nil
		})
		if err != nil {
			return nil, err
		}

		return []remote.Option{remote.WithTransport(transport)}, nil
	}

	return nil, nil
}
