// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ssh

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"golang.org/x/crypto/ssh"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tcache"
)

var (
	sshTransports = tcache.NewCache[*ssh.Client]()
)

type Endpoint struct {
	User           string
	PrivateKeyPath string
	Address        string
}

type Deferred struct {
	CacheKey string
	Dial     func() (*ssh.Client, error)
}

func Establish(ctx context.Context, endpoint Endpoint) (*Deferred, error) {
	debugLogger := console.Debug(ctx)

	if endpoint.User == "" {
		return nil, fnerrors.New("transport.ssh: user is required")
	}

	if endpoint.PrivateKeyPath == "" {
		return nil, fnerrors.New("transport.ssh: private_key_path is required")
	}

	sshAddr := endpoint.Address
	if sshAddr == "" {
		return nil, fnerrors.New("transport.ssh: address is required")
	}

	// XXX use net.SplitHostPort()
	if len(strings.SplitN(sshAddr, ":", 2)) == 1 {
		sshAddr = fmt.Sprintf("%s:22", sshAddr)
	}

	key, err := parsePrivateKey(endpoint.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	config := ssh.ClientConfig{
		User: endpoint.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fmt.Fprintf(debugLogger, "ssh: connected to %q (%s)\n", hostname, remote)
			return nil
		},
	}

	cachekey := fmt.Sprintf("%s:%s@%s", endpoint.User, base64.RawStdEncoding.EncodeToString(key.PublicKey().Marshal()), endpoint.Address)

	return &Deferred{
		CacheKey: cachekey,
		Dial: func() (*ssh.Client, error) {
			return sshTransports.Compute(cachekey, func() (*ssh.Client, error) {
				fmt.Fprintf(debugLogger, "ssh: will dial to %q\n", endpoint.Address)
				return ssh.Dial("tcp", sshAddr, &config)
			})
		},
	}, nil
}

func parsePrivateKey(keyPath string) (ssh.Signer, error) {
	buff, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(buff)
}
