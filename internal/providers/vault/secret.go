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

	if cfg.GetSecretReference() == "" {
		return nil, fnerrors.BadInputError("invalid secret configuration: missing field secret_reference")
	}

	return tasks.Return(ctx, tasks.Action("vault.issue-certificate").Arg("ref", cfg.GetSecretReference()),
		func(ctx context.Context) ([]byte, error) {
			secretSegs := strings.Split(cfg.GetSecretReference(), "/")
			if len(secretSegs) < 3 {
				return nil, fnerrors.BadInputError("invalid vault secret configuration: expects secret_refernece in format '<mount>/<path>/<key>'")
			}

			mount := secretSegs[0]
			path := strings.Join(secretSegs[1:len(secretSegs)-1], "/")
			key := secretSegs[len(secretSegs)-1]

			vaultClient, err := login(ctx, vaultConfig)
			if err != nil {
				return nil, err
			}

			secretResp, err := vaultClient.Secrets.KvV2Read(ctx, path, vaultclient.WithMountPath(mount))
			if err != nil {
				return nil, fnerrors.InvocationError("vault", "failed to read a secret")
			}

			if secretResp.Data.Data == nil {
				return nil, fnerrors.InvocationError("vault", "secret response contained no data")
			}

			secret, ok := secretResp.Data.Data[key].(string)
			if !ok {
				return nil, fnerrors.InvocationError("vault", "response data contained no expected secret %q", key)
			}

			return []byte(secret), nil
		})
}
