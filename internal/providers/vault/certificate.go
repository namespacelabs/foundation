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
	vaultschema "github.com/hashicorp/vault-client-go/schema"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/framework/secrets/combined"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/module"
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
}

func certificateProvider(ctx context.Context, conf cfg.Configuration, secretId secrets.SecretIdentifier, cfg *vault.Certificate) ([]byte, error) {
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

func certificatePemProvider(ctx context.Context, conf cfg.Configuration, secretId secrets.SecretIdentifier, cfg *vault.CertificatePem) ([]byte, error) {
	return extractBundleField(ctx, conf, cfg.GetBundleRef(), func(bundle *vault.TlsBundle) string {
		return bundle.CertificatePem
	})
}

func privateKeyPemProvider(ctx context.Context, conf cfg.Configuration, secretId secrets.SecretIdentifier, cfg *vault.PrivateKeyPem) ([]byte, error) {
	return extractBundleField(ctx, conf, cfg.GetBundleRef(), func(bundle *vault.TlsBundle) string {
		return bundle.PrivateKeyPem
	})
}

func caChainPemProvider(ctx context.Context, conf cfg.Configuration, secretId secrets.SecretIdentifier, cfg *vault.CaChainPem) ([]byte, error) {
	return extractBundleField(ctx, conf, cfg.GetBundleRef(), func(bundle *vault.TlsBundle) string {
		return strings.Join(bundle.CaChainPem, "\n")
	})
}

func issueCertificate(ctx context.Context, vaultClient *vaultclient.Client, pkiMount, pkiRole string, req certificateRequest) ([]byte, error) {
	return tasks.Return(ctx, tasks.Action("vault.issue-certificate").Arg("pki-mount", pkiMount).Arg("pki-role", pkiRole).Arg("common-name", req.commonName),
		func(ctx context.Context) ([]byte, error) {
			issueResp, err := vaultClient.Secrets.PkiIssueWithRole(ctx, pkiRole,
				vaultschema.PkiIssueWithRoleRequest{
					CommonName:        req.commonName,
					AltNames:          strings.Join(req.sans, ","),
					ExcludeCnFromSans: req.excludeCnFromSans,
					IpSans:            req.ipSans,
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

func extractBundleField(ctx context.Context, conf cfg.Configuration, bundleRef string, fn func(*vault.TlsBundle) string) ([]byte, error) {
	ref, err := schema.StrictParsePackageRef(bundleRef)
	if err != nil {
		return nil, fnerrors.BadInputError("could not parse bundle ref %s: %w", bundleRef, err)
	}

	root, err := module.FindRoot(ctx, ".")
	if err != nil {
		return nil, err
	}

	env, err := cfg.LoadContext(root, conf.EnvKey())
	if err != nil {
		return nil, err
	}

	source, err := combined.NewCombinedSecrets(env)
	if err != nil {
		return nil, err
	}

	pl := parsing.NewPackageLoader(env)
	if _, err := pl.LoadByName(ctx, ref.AsPackageName()); err != nil {
		return nil, err
	}

	res, err := source.Load(ctx, pl.Seal(), &secrets.SecretLoadRequest{SecretRef: ref})
	if err != nil {
		return nil, err
	}

	bundle, err := vault.ParseTlsBundle(res.Value)
	if err != nil {
		return nil, err
	}

	return []byte(fn(bundle)), nil
}
