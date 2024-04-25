// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/rs/zerolog"
)

type Credentials struct {
	RoleId   string `json:"role_id"`
	SecretId string `json:"secret_id"`

	VaultAddress   string `json:"vault_address"`
	VaultNamespace string `json:"vault_namespace"`
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

func (c *Credentials) Login(ctx context.Context, options ...vault.ClientOption) (*vault.Client, error) {
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

	resp, err := client.Auth.AppRoleLogin(
		ctx,
		schema.AppRoleLoginRequest{
			RoleId:   c.RoleId,
			SecretId: c.SecretId,
		},
	)
	if err != nil {
		return nil, err
	}
	if err := client.SetToken(resp.Auth.ClientToken); err != nil {
		return nil, err
	}

	go renew(ctx, client, resp.Auth)

	return client, nil
}

func renew(ctx context.Context, client *vault.Client, auth *vault.ResponseAuth) {
	lease := time.Duration(auth.LeaseDuration) * time.Second
	if lease <= 0 {
		return // token does not expire
	}
	if !auth.Renewable {
		zerolog.Ctx(ctx).Warn().Msgf("vault: non-renewable token expires in %s", lease)
		return
	}

	interval := lease - time.Minute*2
	if interval < 0 {
		zerolog.Ctx(ctx).Warn().Msgf("vault: not renewing token, lease too short: %s", lease)
		return
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := client.Auth.TokenRenewSelf(ctx, schema.TokenRenewSelfRequest{}); err != nil {
				// TODO: Let the consumer know, so it could try and reconnect if needed?
				zerolog.Ctx(ctx).Error().Msgf("vault: failed to renew token: %v", err)
				return // TODO: retry, with backoff
			}
		case <-ctx.Done():
			return
		}
	}
}
