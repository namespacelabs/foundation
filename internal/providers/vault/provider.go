// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"fmt"
	"os"

	vaultclient "github.com/hashicorp/vault-client-go"
	"namespacelabs.dev/foundation/framework/secrets/combined"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tcache"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

var (
	vaultConfigType       = cfg.DefineConfigType[*vault.VaultProvider]()
	certificateConfigType = cfg.DefineConfigType[*vault.CertificateConfig]()

	clients = tcache.NewCache[*vaultclient.Client]()
)

func GetVaultConfig(cfg cfg.Configuration) (*vault.VaultProvider, bool) {
	return vaultConfigType.CheckGet(cfg)
}

func GetCertificateConfig(cfg cfg.Configuration) (*vault.CertificateConfig, bool) {
	return certificateConfigType.CheckGet(cfg)
}

func Register() {
	combined.RegisterSecretsProvider(appRoleProvider)
	combined.RegisterSecretsProvider(certificateProvider)
	combined.RegisterSecretsProvider(certificateAuthorityProvider)
	combined.RegisterSecretsProvider(secretProvider)
}

func login(ctx context.Context, vaultCfg *vault.VaultProvider) (*vaultclient.Client, error) {
	key := vaultCfg.GetAddress() + vaultCfg.GetNamespace() + vaultCfg.GetAuthMount()
	return clients.Compute(key, func() (*vaultclient.Client, error) {
		return tasks.Return(ctx, tasks.Action("vault.login").Arg("namespace", vaultCfg.GetNamespace()).Arg("address", vaultCfg.GetAddress()),
			func(ctx context.Context) (*vaultclient.Client, error) {
				client, err := vaultclient.New(
					vaultclient.WithAddress(vaultCfg.GetAddress()),
					vaultclient.WithRequestTimeout(vaultRequestTimeout),
					withIssue257Workaround(),
				)
				if err != nil {
					return nil, fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
				}

				client.SetNamespace(vaultCfg.GetNamespace())

				// Vault by default always prefers a token set in VAULT_TOKEN env var. We do the same.
				// Useful in case of VAULT_TOKEN provided by the 3rd party (e.g. by CI, etc).
				if token := os.Getenv("VAULT_TOKEN"); token != "" {
					fmt.Fprintf(console.Debug(ctx), "skipping login as envroment variable VAULT_TOKEN is set\n")
					client.SetToken(token)
					return client, nil
				}

				var authResp *vaultclient.ResponseAuth
				switch vaultCfg.GetAuthMethod() {
				case "jwt":
					resp, err := vault.JwtLogin(ctx, client, vaultCfg.GetAuthMount(), vault.VaultJwtAudience)
					if err != nil {
						return nil, fnerrors.InvocationError("vault", "failed to login with JWT method: %w", err)
					}

					authResp = resp
				case "oidc":
					resp, err := vault.OidcLogin(ctx, client, vaultCfg.GetAuthMount())
					if err != nil {
						return nil, fnerrors.InvocationError("vault", "failed to login with OIDC method: %w", err)
					}

					authResp = resp
				default:
					return nil, fnerrors.BadDataError("unknown authentication method %q; valid methods: jwt, oidc", vaultCfg.AuthMethod)
				}

				client.SetToken(authResp.ClientToken)
				return client, nil
			},
		)
	})
}
