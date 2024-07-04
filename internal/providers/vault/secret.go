// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"strings"

	vaultclient "github.com/hashicorp/vault-client-go"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

func secretProvider(ctx context.Context, conf cfg.Configuration, secretId secrets.SecretIdentifier, cfg *vault.Secret) ([]byte, error) {
	vaultConfig, ok := GetVaultConfig(conf)
	if !ok || vaultConfig == nil {
		return nil, fnerrors.BadInputError("invalid secrets provider configuration: missing vault configuration")
	}

	secretRef := cfg.GetSecretReference()
	if secretRef == "" {
		secretRef = secretId.SecretRef
	}

	return tasks.Return(ctx, tasks.Action("vault.read-secret").Arg("ref", secretRef),
		func(ctx context.Context) ([]byte, error) {
			secretPkg, secretKey, found := strings.Cut(secretRef, ":")
			if !found {
				return nil, fnerrors.BadInputError("invalid vault secret reference: expects secret refernece in format '<mount>/<path>:<key>'")
			}

			secretMount, secretPath, found := strings.Cut(secretPkg, "/")
			if !found {
				return nil, fnerrors.BadInputError("invalid vault secret package: expects secret package in format '<mount>/<path>'")
			}
			vaultClient, err := login(ctx, vaultConfig)
			if err != nil {
				return nil, err
			}

			secretResp, err := vaultClient.Secrets.KvV2Read(ctx, secretPath, vaultclient.WithMountPath(secretMount))
			if err != nil {
				return nil, fnerrors.InvocationError("vault", "failed to read a secret")
			}

			if secretResp.Data.Data == nil {
				return nil, fnerrors.InvocationError("vault", "secret response contained no data")
			}

			secret, ok := secretResp.Data.Data[secretKey].(string)
			if !ok {
				return nil, fnerrors.InvocationError("vault", "response data contained no expected secret %q", secretKey)
			}

			return []byte(secret), nil
		})
}
