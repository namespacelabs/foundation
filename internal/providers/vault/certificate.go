// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	vaultclient "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/framework/secrets/combined"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

const (
	VaultJwtAudience    = "vault.namespace.systems"
	vaultRequestTimeout = 10 * time.Second
)

func Register() {
	p := &provider{}
	combined.RegisterSecretsProvider(
		func(ctx context.Context, srvRef *secrets.ServerRef, cfg *vault.Certificate) ([]byte, error) {
			ca := cfg.GetCa()
			if ca == nil {
				return nil, fnerrors.BadInputError("invalid vault certificate configuration: missing CA configuration")
			}

			if err := p.Login(ctx, ca, VaultJwtAudience); err != nil {
				return nil, err
			}

			commonName := fmt.Sprintf("%s.%s", strings.ReplaceAll(srvRef.RelPath, "/", "-"), cfg.BaseDomain)
			return p.IssueCertificate(ctx, ca, commonName)
		},
	)
}

type tlsKeyPair struct {
	PrivateKey  string   `json:"private_key"`
	Certificate string   `json:"certificate"`
	CaChain     []string `json:"ca_chain"`
}

type provider struct {
	vaultClient *vaultclient.Client
	once        sync.Once
}

func (p *provider) Login(ctx context.Context, caCfg *vault.CertificateAuthority, audience string) error {
	var rErr error
	p.once.Do(func() {
		if err := tasks.Return0(ctx, tasks.Action("vault.login").Arg("namespace", caCfg.VaultNamespace).Arg("address", caCfg.VaultAddr),
			func(ctx context.Context) error {
				var err error
				p.vaultClient, err = vaultclient.New(
					vaultclient.WithAddress(caCfg.VaultAddr),
					vaultclient.WithRequestTimeout(vaultRequestTimeout),
				)
				if err != nil {
					return fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
				}

				p.vaultClient.SetNamespace(caCfg.VaultNamespace)

				idTokenResp, err := fnapi.IssueIdToken(ctx, audience, 1)
				if err != nil {
					return err
				}

				loginResp, err := p.vaultClient.Auth.JwtLogin(ctx, schema.JwtLoginRequest{Jwt: idTokenResp.IdToken},
					vaultclient.WithMountPath(caCfg.GetAuthMethod()),
				)
				if err != nil {
					return fnerrors.InvocationError("vault", "failed to login to vault: %w", err)
				}

				if loginResp.Auth == nil {
					return fnerrors.InvocationError("vault", "missing vault login auth data: %w", err)
				}

				p.vaultClient.SetToken(loginResp.Auth.ClientToken)
				return nil
			},
		); err != nil {
			rErr = err
		}
	})

	return rErr
}

func (p *provider) IssueCertificate(ctx context.Context, ca *vault.CertificateAuthority, cn string) ([]byte, error) {
	return tasks.Return(ctx, tasks.Action("vault.issue-certificate").Arg("issuer", ca.GetIssuer()).Arg("common-name", cn),
		func(ctx context.Context) ([]byte, error) {
			pkiMount, role, ok := strings.Cut(ca.GetIssuer(), "/")
			if !ok {
				return nil, fnerrors.BadDataError("invalid issuer format; expected <pki-mount>/<role>")
			}

			issueResp, err := p.vaultClient.Secrets.PkiIssueWithRole(ctx, role,
				schema.PkiIssueWithRoleRequest{CommonName: cn},
				vaultclient.WithMountPath(pkiMount),
			)
			if err != nil {
				return nil, fnerrors.InvocationError("vault", "failed to issue a certificate: %w", err)
			}

			cert := tlsKeyPair{
				PrivateKey:  issueResp.Data.PrivateKey,
				Certificate: issueResp.Data.Certificate,
				CaChain:     issueResp.Data.CaChain,
			}

			data, err := json.Marshal(cert)
			if err != nil {
				return nil, fnerrors.BadDataError("failed to serialize certificate data: %w", err)
			}

			return data, nil
		},
	)
}
