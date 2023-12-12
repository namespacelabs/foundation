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

	"github.com/gravitational/teleport/api/client/proxy"
	"github.com/gravitational/teleport/api/profile"
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
	User                string
	PrivateKeyPath      string
	AgentSockPath       string
	Address             string
	TeleportProfileName string
}

type Deferred struct {
	CacheKey string
	Dial     func(context.Context) (*ssh.Client, error)
}

func Establish(ctx context.Context, endpoint Endpoint) (*Deferred, error) {
	debugLogger := console.Debug(ctx)

	if endpoint.User == "" {
		return nil, fnerrors.New("transport.ssh: user is required")
	}

	if endpoint.Address == "" {
		return nil, fnerrors.New("transport.ssh: address is required")
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
	var dialer func(context.Context, string) (net.Conn, error)

	if endpoint.TeleportProfileName != "" {
		if endpoint.AgentSockPath != "" {
			return nil, fnerrors.New("ssh: can't use both teleport_profile_name and agent_sock_path")
		}

		if endpoint.PrivateKeyPath != "" {
			return nil, fnerrors.New("ssh: can't use private_key_path with teleport_profile_name")
		}

		p, err := profile.FromDir("", endpoint.TeleportProfileName)
		if err != nil {
			return nil, err
		}

		tlscfg, err := p.TLSConfig()
		if err != nil {
			return nil, err
		}

		sshcfg, err := p.SSHClientConfig()
		if err != nil {
			return nil, err
		}

		clt, err := proxy.NewClient(ctx, proxy.ClientConfig{
			ProxyAddress:            p.WebProxyAddr,
			TLSRoutingEnabled:       p.TLSRoutingEnabled,
			TLSConfig:               tlscfg,
			SSHConfig:               sshcfg,
			ALPNConnUpgradeRequired: p.TLSRoutingConnUpgradeRequired && p.TLSRoutingEnabled,
		})
		if err != nil {
			return nil, err
		}

		// Hardcoded default port. See https://github.com/gravitational/teleport/blob/da589355d4ea55de276062db09f440c6fefdb2d6/lib/defaults/defaults.go#L48
		sshPort = "3022"

		fmt.Fprintf(debugLogger, "ssh: using teleport via %q\n", p.SiteName)

		config = sshcfg
		dialer = func(ctx context.Context, addr string) (net.Conn, error) {
			conn, _, err := clt.DialHost(ctx, addr, p.SiteName, nil)
			return conn, err
		}
	} else {
		config = &ssh.ClientConfig{
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				fmt.Fprintf(debugLogger, "ssh: connected to %q (%s)\n", hostname, remote)
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
				return nil, fnerrors.New("failed to resolve ssh agent path: %w", err)
			}

			keyKey += ":agent=" + path

			conn, err := net.Dial("unix", path)
			if err != nil {
				return nil, fnerrors.New("failed to connect to ssh agent: %w", err)
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
				fmt.Fprintf(debugLogger, "ssh: will dial to %s:%s\n", sshAddr, sshPort)

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
