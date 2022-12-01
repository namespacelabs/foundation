// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package oci

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/remote"
	"golang.org/x/crypto/ssh"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tcache"
)

var (
	sshTransports = tcache.NewCache[*http.Transport]()
)

func parseTransport(ctx context.Context, t *registry.RegistryTransport) ([]remote.Option, error) {
	if t == nil {
		return nil, nil
	}

	debugLogger := console.Debug(ctx)

	switch {
	case t.Ssh != nil:
		if t.Ssh.User == "" {
			return nil, fnerrors.New("transport.ssh: user is required")
		}

		if t.Ssh.PrivateKeyPath == "" {
			return nil, fnerrors.New("transport.ssh: private_key_path is required")
		}

		sshAddr := t.Ssh.SshAddr
		if sshAddr == "" {
			return nil, fnerrors.New("transport.ssh: ssh_addr is required")
		}

		// XXX use net.SplitHostPort()
		if len(strings.SplitN(sshAddr, ":", 2)) == 1 {
			sshAddr = fmt.Sprintf("%s:22", sshAddr)
		}

		remoteAddr := t.Ssh.RemoteAddr
		if remoteAddr == "" {
			return nil, fnerrors.New("transport.ssh: missing remote address")
		}

		key, err := parsePrivateKey(t.Ssh.PrivateKeyPath)
		if err != nil {
			return nil, err
		}

		config := ssh.ClientConfig{
			User: t.Ssh.User,
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(key),
			},
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				fmt.Fprintf(debugLogger, "ssh: connected to %q (%s)\n", hostname, remote)
				return nil
			},
		}

		cachekey := fmt.Sprintf("%s:%s@%s", t.Ssh.User, base64.RawStdEncoding.EncodeToString(key.PublicKey().Marshal()), t.Ssh.SshAddr)

		transport, err := sshTransports.Compute(cachekey, func() (*http.Transport, error) {
			fmt.Fprintf(debugLogger, "ssh: will dial to %q to get to %q\n", t.Ssh.SshAddr, t.Ssh.RemoteAddr)
			conn, err := ssh.Dial("tcp", sshAddr, &config)
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

func parsePrivateKey(keyPath string) (ssh.Signer, error) {
	buff, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(buff)
}
