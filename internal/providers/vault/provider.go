// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"

	vaultclient "github.com/hashicorp/vault-client-go"
	"namespacelabs.dev/foundation/framework/secrets/combined"

	"github.com/hashicorp/vault-client-go/schema"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tcache"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

var clients = tcache.NewCache[*vaultclient.Client]()

func Register() {
	combined.RegisterSecretsProvider(appRoleProvider)
	combined.RegisterSecretsProvider(certificateProvider)
}

func login(ctx context.Context, caCfg *vault.VaultProvider, audience string) (*vaultclient.Client, error) {
	key := caCfg.GetAddress() + caCfg.GetNamespace() + caCfg.GetAuthMount()
	return clients.Compute(key, func() (*vaultclient.Client, error) {
		return tasks.Return(ctx, tasks.Action("vault.login").Arg("namespace", caCfg.GetNamespace()).Arg("address", caCfg.GetAddress()),
			func(ctx context.Context) (*vaultclient.Client, error) {
				client, err := vaultclient.New(
					vaultclient.WithAddress(caCfg.GetAddress()),
					vaultclient.WithRequestTimeout(vaultRequestTimeout),
					withIssue257Workaround(),
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
					vaultclient.WithMountPath(caCfg.GetAuthMount()),
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
	})
}
