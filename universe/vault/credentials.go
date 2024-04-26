// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/rs/zerolog"
)

// Force re-authentication if the token expires in less than this much time.
const ttlBuffer = time.Second * 8

type Credentials struct {
	RoleId   string `json:"role_id"`
	SecretId string `json:"secret_id"`

	VaultAddress   string `json:"vault_address"`
	VaultNamespace string `json:"vault_namespace"`
}

type ClientHandle struct {
	creds  *Credentials
	client *vault.Client
	auth   *vault.ResponseAuth
	leased time.Time

	m sync.Mutex
}

func ParseCredentials(data []byte) (*Credentials, error) {
	c := Credentials{}
	return &c, json.Unmarshal(data, &c)
}

func ParseCredentialsFromEnv(key string) (*Credentials, error) {
	return ParseCredentials([]byte(os.Getenv(key)))
}

func (c *Credentials) Encode() ([]byte, error) {
	return json.Marshal(c)
}

func (c *Credentials) ClientHandle(ctx context.Context, options ...vault.ClientOption) (*ClientHandle, error) {
	client, err := vault.New(append([]vault.ClientOption{
		vault.WithAddress(c.VaultAddress),
	}, options...)...)
	if err != nil {
		return nil, err
	}

	if c.VaultNamespace != "" {
		if err := client.SetNamespace(c.VaultNamespace); err != nil {
			return nil, err
		}
	}

	return &ClientHandle{
		creds:  c,
		client: client,
	}, nil
}

func (h *ClientHandle) Get(ctx context.Context) (*vault.Client, error) {
	h.m.Lock()
	defer h.m.Unlock()

	// Re-authenticate if less than six seconds left.
	if time.Now().Add(ttlBuffer).After(h.expires()) {
		return h.client, h.renew(ctx)
	}

	return h.client, nil
}

func (h *ClientHandle) authenticate(ctx context.Context) error {
	resp, err := h.client.Auth.AppRoleLogin(
		ctx,
		schema.AppRoleLoginRequest{
			RoleId:   h.creds.RoleId,
			SecretId: h.creds.SecretId,
		},
	)
	if err != nil {
		return err
	}
	h.auth = resp.Auth
	h.leased = time.Now()
	zerolog.Ctx(ctx).Debug().Dur("lease_duration", h.ttl()).Msg("vault: authenticated")
	return h.client.SetToken(resp.Auth.ClientToken)
}

func (h *ClientHandle) renew(ctx context.Context) error {
	if h.auth == nil || !h.auth.Renewable {
		return h.authenticate(ctx)
	}

	res, err := h.client.Auth.TokenRenewSelf(ctx, schema.TokenRenewSelfRequest{})
	if err != nil {
		// The Vault client library already handles retries, so if renewing the
		// token fails, we assume it can no longer be renewed. This can happen if
		// the token was revoked, or if it reached its maximum TTL.
		zerolog.Ctx(ctx).Warn().Msg("vault: token renewal failed, forcing re-auth")
		return h.authenticate(ctx)
	}
	h.auth = res.Auth
	zerolog.Ctx(ctx).Debug().Dur("lease_duration", h.ttl()).Msg("vault: token renewed")
	h.leased = time.Now()
	return nil
}

func (h *ClientHandle) ttl() time.Duration {
	if h.auth == nil {
		return 0
	}
	return time.Duration(h.auth.LeaseDuration) * time.Second
}

func (h *ClientHandle) expires() time.Time {
	if h.auth == nil {
		return time.Time{}
	}
	return h.leased.Add(h.ttl())
}
