// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"os"

	vaultclient "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"namespacelabs.dev/foundation/framework/secrets/combined"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tcache"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

const VaulTokenEnvKey = "VAULT_TOKEN"

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

				// Vault by default always prefers a token set in VAULT_TOKEN env var. We do the same.
				// Useful in case of VAULT_TOKEN provided by the 3rd party (e.g. by CI, etc).
				vaultToken := os.Getenv(VaulTokenEnvKey)
				if vaultToken != "" {
					client.SetToken(vaultToken)
					return client, nil
				}

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
