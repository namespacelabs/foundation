// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func NewIssueClientCertCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "issue-client-cert",
		Short: "Issue a client certificate for the current Namespace authentication token.",
		Args:  cobra.NoArgs,
	}

	publicKeyPath := cmd.Flags().String("public_key_file", "", "If specified, use this PEM-encoded public key instead of generating one.")
	outputPrivateKeyPath := cmd.Flags().String("output_private_key_file", "", "If specified, write the generated private key PEM to this path.")
	outputPublicKeyPath := cmd.Flags().String("output_public_key_file", "", "If specified, write the generated public key PEM to this path.")

	return fncobra.Cmd(cmd).Do(func(ctx context.Context) error {
		var privateKeyPem []byte

		publicKeyPem, privateKeyPem, err := loadOrCreatePublicKey(*publicKeyPath)
		if err != nil {
			return err
		}

		if *publicKeyPath != "" && *outputPrivateKeyPath != "" {
			return fnerrors.Newf("--output_private_key_file cannot be used with --public_key_file")
		}

		if err := writePemFile(*outputPrivateKeyPath, privateKeyPem, 0600); err != nil {
			return err
		}

		if err := writePemFile(*outputPublicKeyPath, publicKeyPem, 0644); err != nil {
			return err
		}

		tok, err := fnapi.FetchToken(ctx)
		if err != nil {
			return err
		}

		clientCertPem, err := fnapi.IssueTenantClientCertFromToken(ctx, tok, string(publicKeyPem))
		if err != nil {
			return err
		}

		if *publicKeyPath == "" && *outputPrivateKeyPath == "" && *outputPublicKeyPath == "" {
			fmt.Fprint(console.Stdout(ctx), string(privateKeyPem))
			if !strings.HasSuffix(string(privateKeyPem), "\n") {
				fmt.Fprintln(console.Stdout(ctx))
			}
		}

		fmt.Fprint(console.Stdout(ctx), clientCertPem)
		if !strings.HasSuffix(clientCertPem, "\n") {
			fmt.Fprintln(console.Stdout(ctx))
		}

		return nil
	})
}

func loadOrCreatePublicKey(path string) ([]byte, []byte, error) {
	if path != "" {
		publicKeyPem, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fnerrors.Newf("failed to read %q: %w", path, err)
		}

		return publicKeyPem, nil, nil
	}

	privateKeyPem, publicKeyPem, err := generateClientKeyPair()
	if err != nil {
		return nil, nil, err
	}

	return publicKeyPem, privateKeyPem, nil
}

func writePemFile(path string, contents []byte, mode os.FileMode) error {
	if path == "" {
		return nil
	}

	if err := os.WriteFile(path, contents, mode); err != nil {
		return fnerrors.Newf("failed to write %q: %w", path, err)
	}

	return nil
}

func generateClientKeyPair() ([]byte, []byte, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fnerrors.Newf("failed to generate private key: %w", err)
	}

	publicKeyDer, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, fnerrors.Newf("failed to marshal public key: %w", err)
	}

	privateKeyDer, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return nil, nil, fnerrors.Newf("failed to marshal private key: %w", err)
	}

	privateKeyPem := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privateKeyDer})
	publicKeyPem := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDer})

	return privateKeyPem, publicKeyPem, nil
}
