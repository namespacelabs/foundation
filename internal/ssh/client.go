// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ssh

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tcache"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

var (
	sshTransports = tcache.NewCache[*ssh.Client]()
)

type Endpoint struct {
	User           string
	PrivateKeyPath string
	AgentSockPath  string
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

	sshAddr := endpoint.Address
	if sshAddr == "" {
		return nil, fnerrors.New("transport.ssh: address is required")
	}

	// XXX use net.SplitHostPort()
	if len(strings.SplitN(sshAddr, ":", 2)) == 1 {
		sshAddr = fmt.Sprintf("%s:22", sshAddr)
	}

	var auths []ssh.AuthMethod

	key, keyKey, err := parseAuth(endpoint.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	if key != nil {
		auths = append(auths, key)
	}

	if endpoint.AgentSockPath != "" {
		path, err := dirs.ExpandHome(os.ExpandEnv(endpoint.AgentSockPath))
		if err != nil {
			return nil, fnerrors.New("failed to resolve ssh agent path: %w", err)
		}

		keyKey += ":agent=" + path

		conn, err := net.Dial("unix", path)
		if err != nil {
			return nil, fnerrors.New("failed to connect to ssh agent: %w", err)
		}

		agentClient := agent.NewClient(conn)
		auths = append(auths, ssh.PublicKeysCallback(agentClient.Signers))
	}

	config := ssh.ClientConfig{
		User: endpoint.User,
		Auth: auths,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fmt.Fprintf(debugLogger, "ssh: connected to %q (%s)\n", hostname, remote)
			return nil
		},
	}

	cachekey := fmt.Sprintf("%s:%s@%s", endpoint.User, keyKey, endpoint.Address)

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

func parseAuth(privateKeyPath string) (ssh.AuthMethod, string, error) {
	if privateKeyPath != "" {
		key, err := parsePrivateKey(privateKeyPath)
		if err != nil {
			return nil, "", err
		}

		keyKey := base64.RawStdEncoding.EncodeToString(key.PublicKey().Marshal())
		return ssh.PublicKeys(key), keyKey, nil
	}

	return nil, "", nil
}

func parsePrivateKey(keyPath string) (ssh.Signer, error) {
	buff, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(buff)
}
