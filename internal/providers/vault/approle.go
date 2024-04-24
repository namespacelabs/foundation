// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"errors"

	vaultclient "github.com/hashicorp/vault-client-go"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

func (p *provider) appRoleProvider(ctx context.Context, secretId secrets.SecretIdentifier, cfg *vault.AppRole) ([]byte, error) {
	vp := cfg.GetProvider()
	if vp == nil {
		return nil, fnerrors.BadInputError("invalid vault certificate configuration: missing provider configuration")
	}

	vaultClient, err := p.Login(ctx, vp, vaultJwtAudience)
	if err != nil {
		return nil, err
	}

	if secretId.ServerRef == nil {
		return nil, fnerrors.BadDataError("required server reference is not set")
	}

	return p.CreateSecretId(ctx, vaultClient, cfg.GetName())
}

func (p *provider) CreateSecretId(ctx context.Context, vaultClient *vaultclient.Client, name string) ([]byte, error) {
	return tasks.Return(ctx, tasks.Action("vault.create-secret-id").Arg("name", name),
		func(ctx context.Context) ([]byte, error) {
			return nil, errors.New("todo: not implemented")
		},
	)
}
