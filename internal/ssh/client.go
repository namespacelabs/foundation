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

type DialFunc func(context.Context, string) (net.Conn, error)

type Endpoint struct {
	User           string
	PrivateKeyPath string
	AgentSockPath  string
	Address        string
	TeleportProxy  *TeleportProxy
}

type TeleportProxy struct {
	ProfileName     string
	Host            string
	TbotIdentityDir string
	ProxyAddress    string
	Cluster         string
}

type Deferred struct {
	CacheKey string
	Dial     func(context.Context) (*ssh.Client, error)
}

func Establish(ctx context.Context, endpoint Endpoint) (*Deferred, error) {
	if endpoint.User == "" {
		return nil, fnerrors.Newf("transport.ssh: user is required")
	}

	if endpoint.Address == "" {
		return nil, fnerrors.Newf("transport.ssh: address is required")
	}

	sshAddr, sshPort, err := net.SplitHostPort(endpoint.Address)
	if err != nil {
		sshAddr = endpoint.Address
		sshPort = "22"
	}

	key, keyKey, err := parseAuth(endpoint.PrivateKeyPath)
	if err != nil {
		return nil, err
	}

	var config *ssh.ClientConfig
	var dialer DialFunc

	if teleportProxy := endpoint.TeleportProxy; teleportProxy != nil {
		return nil, fnerrors.Newf("transport.ssh: teleport not supported")
	} else {
		config = &ssh.ClientConfig{
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				fmt.Fprintf(console.Debug(ctx), "ssh: connected to %q (%s)\n", hostname, remote)
				return nil
			},
		}

		dialer = func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", addr)
		}

		if key != nil {
			config.Auth = append(config.Auth, key)
		}

		if endpoint.AgentSockPath != "" {
			path, err := dirs.ExpandHome(os.ExpandEnv(endpoint.AgentSockPath))
			if err != nil {
				return nil, fnerrors.Newf("failed to resolve ssh agent path: %w", err)
			}

			keyKey += ":agent=" + path

			conn, err := net.Dial("unix", path)
			if err != nil {
				return nil, fnerrors.Newf("failed to connect to ssh agent: %w", err)
			}

			agentClient := agent.NewClient(conn)
			config.Auth = append(config.Auth, ssh.PublicKeysCallback(agentClient.Signers))
		}
	}

	config.User = endpoint.User

	cachekey := fmt.Sprintf("%s:%s@%s:%s", endpoint.User, keyKey, sshAddr, sshPort)

	return &Deferred{
		CacheKey: cachekey,
		Dial: func(ctx context.Context) (*ssh.Client, error) {
			return sshTransports.Compute(cachekey, func() (*ssh.Client, error) {
				fmt.Fprintf(console.Debug(ctx), "ssh: will dial to %s:%s\n", sshAddr, sshPort)

				addrport := fmt.Sprintf("%s:%s", sshAddr, sshPort)
				conn, err := dialer(ctx, addrport)
				if err != nil {
					return nil, err
				}

				c, chans, reqs, err := ssh.NewClientConn(conn, addrport, config)
				if err != nil {
					return nil, err
				}

				return ssh.NewClient(c, chans, reqs), nil
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
