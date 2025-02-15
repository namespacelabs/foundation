// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"

	vaultclient "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/executor"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/std/tryhard"
	"namespacelabs.dev/foundation/universe/vault"
)

func appRoleProvider(ctx context.Context, conf cfg.Configuration, _ *secrets.SecretLoadRequest, cfg *vault.AppRole) ([]byte, error) {
	vaultConfig, ok := GetVaultConfig(conf)
	if !ok || vaultConfig == nil {
		return nil, fnerrors.BadInputError("invalid vault app role configuration: missing provider configuration")
	}

	vaultClient, err := login(ctx, vaultConfig)
	if err != nil {
		return nil, err
	}

	return createSecretId(ctx, vaultClient, vaultConfig, cfg)
}

func createSecretId(ctx context.Context, vaultClient *vaultclient.Client, vaultConfig *vault.VaultProvider, cfg *vault.AppRole) ([]byte, error) {
	creds := vault.Credentials{
		VaultAddress:   vaultConfig.GetAddress(),
		VaultNamespace: vaultConfig.GetNamespace(),
	}
	wmp := vaultclient.WithMountPath(cfg.GetMount())

	ex := executor.New(ctx, "vault.credentials")
	ex.Go(func(ctx context.Context) error {
		var err error
		creds.RoleId, err = tasks.Return(ctx, tasks.Action("vault.read-role-id").Arg("name", cfg.GetName()),
			func(ctx context.Context) (string, error) {
				return tryhard.CallSideEffectFree1(ctx, true, func(ctx context.Context) (string, error) {
					res, err := vaultClient.Auth.AppRoleReadRoleId(ctx, cfg.GetName(), wmp)
					if err != nil {
						return "", fnerrors.InvocationError("vault", "failed to read role id: %w", err)
					}
					return res.Data.RoleId, nil
				})
			})
		return err
	})
	ex.Go(func(ctx context.Context) error {
		var err error
		creds.SecretId, err = tasks.Return(ctx, tasks.Action("vault.create-secret-id").Arg("name", cfg.GetName()),
			func(context.Context) (string, error) {
				res, err := vaultClient.Auth.AppRoleWriteSecretId(ctx, cfg.GetName(), schema.AppRoleWriteSecretIdRequest{}, wmp)
				if err != nil {
					return "", fnerrors.InvocationError("vault", "failed to create secret id: %w", err)
				}
				return res.Data.SecretId, nil
			})
		return err
	})

	if err := ex.Wait(); err != nil {
		return nil, err
	}

	data, err := creds.Encode()
	if err != nil {
		return nil, fnerrors.BadDataError("failed to serialize credentials: %w", err)
	}

	return data, nil
}
