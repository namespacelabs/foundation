// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"fmt"
	"strings"
	"time"

	vaultclient "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

const (
	vaultJwtAudience    = "vault.namespace.systems"
	vaultRequestTimeout = 10 * time.Second
)

func (p *provider) certificateProvider(ctx context.Context, secretId secrets.SecretIdentifier, cfg *vault.Certificate) ([]byte, error) {
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

	commonName := fmt.Sprintf("%s.%s", strings.ReplaceAll(secretId.ServerRef.RelPath, "/", "-"), cfg.GetBaseDomain())
	return p.IssueCertificate(ctx, vaultClient, cfg.GetIssuer(), commonName)
}

type vaultIdentifier struct {
	address    string
	namespace  string
	authMethod string
}

func (v vaultIdentifier) String() string {
	return v.address + v.namespace + v.authMethod
}

func (p *provider) IssueCertificate(ctx context.Context, vaultClient *vaultclient.Client, issuer, cn string) ([]byte, error) {
	return tasks.Return(ctx, tasks.Action("vault.issue-certificate").Arg("issuer", issuer).Arg("common-name", cn),
		func(ctx context.Context) ([]byte, error) {
			pkiMount, role, ok := strings.Cut(issuer, "/")
			if !ok {
				return nil, fnerrors.BadDataError("invalid issuer format; expected <pki-mount>/<role>")
			}

			issueResp, err := vaultClient.Secrets.PkiIssueWithRole(ctx, role,
				schema.PkiIssueWithRoleRequest{CommonName: cn},
				vaultclient.WithMountPath(pkiMount),
			)
			if err != nil {
				return nil, fnerrors.InvocationError("vault", "failed to issue a certificate: %w", err)
			}

			data, err := vault.TlsBundle{
				PrivateKeyPem:  issueResp.Data.PrivateKey,
				CertificatePem: issueResp.Data.Certificate,
				CaChainPem:     issueResp.Data.CaChain,
			}.Encode()
			if err != nil {
				return nil, fnerrors.BadDataError("failed to serialize certificate data: %w", err)
			}

			return data, nil
		},
	)
}
