// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"sync"

	vaultclient "github.com/hashicorp/vault-client-go"
	"namespacelabs.dev/foundation/framework/secrets/combined"

	"github.com/hashicorp/vault-client-go/schema"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

func Register() {
	p := &provider{
		vaultClients: make(map[string]*vaultclient.Client),
	}
	combined.RegisterSecretsProvider(p.appRoleProvider)
	combined.RegisterSecretsProvider(p.certificateProvider)
}

type provider struct {
	mtx          sync.Mutex
	vaultClients map[string]*vaultclient.Client
}

func (p *provider) Login(ctx context.Context, caCfg *vault.VaultProvider, audience string) (*vaultclient.Client, error) {
	vKey := vaultIdentifier{
		address:    caCfg.GetAddress(),
		namespace:  caCfg.GetNamespace(),
		authMethod: caCfg.GetAuthMethod(),
	}
	p.mtx.Lock()
	defer p.mtx.Unlock()
	client, ok := p.vaultClients[vKey.String()]
	if ok {
		return client, nil
	}

	client, err := tasks.Return(ctx, tasks.Action("vault.login").Arg("namespace", caCfg.GetNamespace()).Arg("address", caCfg.GetAddress()),
		func(ctx context.Context) (*vaultclient.Client, error) {
			client, err := vaultclient.New(
				vaultclient.WithAddress(caCfg.GetAddress()),
				vaultclient.WithRequestTimeout(vaultRequestTimeout),
			)
			if err != nil {
				return nil, fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
			}

			client.SetNamespace(caCfg.GetNamespace())

			idTokenResp, err := fnapi.IssueIdToken(ctx, audience, 1)
			if err != nil {
				return nil, err
			}

			loginResp, err := client.Auth.JwtLogin(ctx, schema.JwtLoginRequest{Jwt: idTokenResp.IdToken},
				vaultclient.WithMountPath(caCfg.GetAuthMethod()),
			)
			if err != nil {
				return nil, fnerrors.InvocationError("vault", "failed to login to vault: %w", err)
			}

			if loginResp.Auth == nil {
				return nil, fnerrors.InvocationError("vault", "missing vault login auth data: %w", err)
			}

			client.SetToken(loginResp.Auth.ClientToken)
			return client, nil
		},
	)
	if err != nil {
		return nil, err
	}

	p.vaultClients[vKey.String()] = client

	return client, nil
}
