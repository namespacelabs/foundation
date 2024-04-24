// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"

	vaultclient "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"golang.org/x/sync/errgroup"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

func (p *provider) appRoleProvider(ctx context.Context, secretId secrets.SecretIdentifier, cfg *vault.AppRole) ([]byte, error) {
	vp := cfg.GetProvider()
	if vp == nil {
		return nil, fnerrors.BadInputError("invalid vault app role configuration: missing provider configuration")
	}

	vaultClient, err := p.Login(ctx, vp, vaultJwtAudience)
	if err != nil {
		return nil, err
	}

	if secretId.ServerRef == nil {
		return nil, fnerrors.BadDataError("required server reference is not set")
	}

	return p.CreateSecretId(ctx, vaultClient, cfg)
}

func (p *provider) CreateSecretId(ctx context.Context, vaultClient *vaultclient.Client, cfg *vault.AppRole) ([]byte, error) {
	return tasks.Return(ctx, tasks.Action("vault.create-secret-id").Arg("name", cfg.GetName()),
		func(ctx context.Context) ([]byte, error) {
			creds := vault.Credentials{
				VaultAddress:   cfg.Provider.GetAddress(),
				VaultNamespace: cfg.Provider.GetNamespace(),
			}
			wmp := vaultclient.WithMountPath(cfg.GetAuthMethod())

			g := errgroup.Group{}
			g.Go(func() error {
				res, err := vaultClient.Auth.AppRoleReadRoleId(ctx, cfg.GetName(), wmp)
				if err != nil {
					return fnerrors.InvocationError("vault", "failed to read role id: %w", err)
				}
				creds.RoleId = res.Data.RoleId
				return nil
			})
			g.Go(func() error {
				res, err := vaultClient.Auth.AppRoleWriteSecretId(ctx, cfg.GetName(), schema.AppRoleWriteSecretIdRequest{}, wmp)
				if err != nil {
					return fnerrors.InvocationError("vault", "failed to create secret id: %w", err)
				}
				creds.SecretId = res.Data.SecretId
				return nil
			})
			if err := g.Wait(); err != nil {
				return nil, err
			}

			data, err := creds.Encode()
			if err != nil {
				return nil, fnerrors.BadDataError("failed to serialize credentials: %w", err)
			}

			return data, nil
		},
	)
}
