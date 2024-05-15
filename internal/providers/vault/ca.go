// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"time"

	vaultclient "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
)

func certificateAuthorityProvider(ctx context.Context, conf cfg.Configuration, secretId secrets.SecretIdentifier, cfg *vault.CertificateAuthority) ([]byte, error) {
	vaultConfig, ok := GetVaultConfig(conf)
	if !ok || vaultConfig == nil {
		return nil, fnerrors.BadInputError("invalid vault configuration: missing provider configuration")
	}

	vaultClient, err := login(ctx, vaultConfig)
	if err != nil {
		return nil, err
	}

	if cfg.CommonName == "" {
		return nil, fnerrors.BadInputError("invalid certificate authority configuration: missing common name")
	}
	if cfg.Ttl == "" {
		return nil, fnerrors.BadInputError("invalid certificate authority configuration: missing ttl")
	}

	if _, err := time.ParseDuration(cfg.Ttl); err != nil {
		return nil, fnerrors.BadInputError("ttl is not a valid duration: %w", err)
	}

	return tasks.Return(ctx, tasks.Action("vault.generate-intermediate-ca").Arg("common_name", cfg.CommonName),
		func(ctx context.Context) ([]byte, error) {
			privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			if err != nil {
				return nil, err
			}

			template := x509.CertificateRequest{
				Subject: pkix.Name{
					CommonName:   cfg.CommonName,
					Organization: cfg.Organization,
				},
				SignatureAlgorithm: x509.ECDSAWithSHA256,
			}

			csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &template, privKey)
			if err != nil {
				return nil, err
			}

			csrPem := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
			privKeyDer, err := x509.MarshalECPrivateKey(privKey)
			if err != nil {
				return nil, err
			}

			privKeyPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privKeyDer})

			intCaResp, err := vaultClient.Secrets.PkiRootSignIntermediate(ctx, schema.PkiRootSignIntermediateRequest{
				CommonName:   cfg.CommonName,
				Csr:          string(csrPem),
				Ttl:          cfg.Ttl,
				Organization: cfg.Organization,
			}, vaultclient.WithMountPath(cfg.Mount))
			if err != nil {
				return nil, fnerrors.InvocationError("vault", "failed to sign intermediate CA: %w", err)
			}

			tb := vault.TlsBundle{
				PrivateKeyPem:  string(privKeyPem),
				CertificatePem: intCaResp.Data.Certificate,
			}

			return tb.Encode()
		},
	)
}
