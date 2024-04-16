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
	vaultJwtAudience    = "vault.namespace.systems"
	vaultRequestTimeout = 10 * time.Second
)

func Register() {
	p := &provider{
		vaultClients: make(map[vaultAuth]*vaultclient.Client),
	}
	combined.RegisterSecretsProvider(
		func(ctx context.Context, srvRef *secrets.ServerRef, cfg *vault.Certificate) ([]byte, error) {
			ca := cfg.GetCa()
			if ca == nil {
				return nil, fnerrors.BadInputError("invalid vault certificate configuration: missing CA configuration")
			}

			vaultClient, err := p.Login(ctx, ca, vaultJwtAudience)
			if err != nil {
				return nil, err
			}

			commonName := fmt.Sprintf("%s.%s", strings.ReplaceAll(srvRef.RelPath, "/", "-"), cfg.BaseDomain)
			return p.IssueCertificate(ctx, vaultClient, ca.GetIssuer(), commonName)
		},
	)
}

type vaultAuth struct {
	addr       string
	authMethod string
}

type tlsKeyPair struct {
	PrivateKey  string   `json:"private_key"`
	Certificate string   `json:"certificate"`
	CaChain     []string `json:"ca_chain"`
}

type provider struct {
	mtx          sync.Mutex
	vaultClients map[vaultAuth]*vaultclient.Client
}

func (p *provider) Login(ctx context.Context, caCfg *vault.CertificateAuthority, audience string) (*vaultclient.Client, error) {
	p.mtx.Lock()
	defer p.mtx.Unlock()

	client, ok := p.vaultClients[vaultAuth{caCfg.GetVaultAddr(), caCfg.GetAuthMethod()}]
	if ok {
		return client, nil
	}

	client, err := tasks.Return(ctx, tasks.Action("vault.login").Arg("namespace", caCfg.VaultNamespace).Arg("address", caCfg.VaultAddr),
		func(ctx context.Context) (*vaultclient.Client, error) {
			client, err := vaultclient.New(
				vaultclient.WithAddress(caCfg.VaultAddr),
				vaultclient.WithRequestTimeout(vaultRequestTimeout),
			)
			if err != nil {
				return nil, fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
			}

			client.SetNamespace(caCfg.VaultNamespace)

			idTokenResp, err := fnapi.IssueIdToken(ctx, audience, 1)
			if err != nil {
				return nil, err
			}

			loginResp, err := client.Auth.JwtLogin(ctx, schema.JwtLoginRequest{Jwt: idTokenResp.IdToken},
				vaultclient.WithMountPath(caCfg.GetAuthMethod()),
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
	if err != nil {
		return nil, err
	}

	p.vaultClients[vaultAuth{caCfg.GetVaultAddr(), caCfg.GetAuthMethod()}] = client

	return client, nil
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
