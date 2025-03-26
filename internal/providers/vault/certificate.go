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
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

const (
	vaultRequestTimeout = 10 * time.Second
)

type certificateRequest struct {
	commonName        string
	sans              []string
	ipSans            []string
	excludeCnFromSans bool
	ttl               string
}

func certificateProvider(ctx context.Context, conf cfg.Configuration, _ *secrets.SecretLoadRequest, cfg *vault.Certificate) ([]byte, error) {
	vaultConfig, ok := GetVaultConfig(conf)
	if !ok || vaultConfig == nil {
		return nil, fnerrors.BadInputError("invalid certificate provider: missing vault configuration")
	}

	if cfg.GetCommonName() == "" {
		return nil, fnerrors.BadInputError("required common name is not set")
	}

	vaultClient, err := login(ctx, vaultConfig)
	if err != nil {
		return nil, err
	}

	req := certificateRequest{
		commonName: cfg.GetCommonName(),
		sans:       cfg.GetSans(),
		ipSans:     cfg.GetIpSans(),
		ttl:        cfg.GetTtl(),
	}

	if certConfig, ok := GetCertificateConfig(conf); ok {
		if base := certConfig.GetBaseDomain(); base != "" {
			req.commonName = fmt.Sprintf("%s/%s", base, req.commonName)
			req.excludeCnFromSans = true
		}

		for _, san := range cfg.GetSans() {
			if base := certConfig.GetBaseDomain(); base != "" {
				req.sans = append(req.sans, fmt.Sprintf("%s.%s", san, base))
			}

			for _, domain := range certConfig.GetSansDomains() {
				req.sans = append(req.sans, fmt.Sprintf("%s.%s", san, domain))
			}
		}
	}

	return issueCertificate(ctx, vaultClient, cfg.GetMount(), cfg.GetRole(), req)
}

func issueCertificate(ctx context.Context, vaultClient *vaultclient.Client, pkiMount, pkiRole string, req certificateRequest) ([]byte, error) {
	return tasks.Return(ctx, tasks.Action("vault.issue-certificate").Arg("pki-mount", pkiMount).Arg("pki-role", pkiRole).Arg("common-name", req.commonName),
		func(ctx context.Context) ([]byte, error) {
			issueResp, err := vaultClient.Secrets.PkiIssueWithRole(ctx, pkiRole,
				schema.PkiIssueWithRoleRequest{
					CommonName:        req.commonName,
					AltNames:          strings.Join(req.sans, ","),
					ExcludeCnFromSans: req.excludeCnFromSans,
					IpSans:            req.ipSans,
					Ttl:               req.ttl,
				},
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
