// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package admin

import (
	"bytes"
	"context"
	"crypto"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	partnerOIDCTokenPrefix = "oidc_"
	tokenAudience          = "namespace.so"
)

func newSignPartnerTokenCmd() *cobra.Command {
	var (
		issuerURL, partnerID   string
		keyFromFile, keyFrom1P string
		signingMethod          string
		tokenDuration          time.Duration
	)

	return fncobra.Cmd(
		&cobra.Command{
			Use:   "sign-partner-token --partner_id=user_... --issuer_url=https://... --key_from_file=...",
			Short: "Run gcloud.",
		}).
		WithFlags(func(flags *pflag.FlagSet) {
			flags.StringVar(&partnerID, "partner_id", "", "Partner account ID to assert.")
			flags.StringVar(&issuerURL, "issuer_url", "", "Issuer URL trusted by Namespace for the partner account")
			flags.StringVar(&signingMethod, "signing_method", "ES256", "JWT signing method (supported: ES256, RS256).")
			flags.StringVar(&keyFromFile, "key_from_file", "", "Read signing key from file.")
			flags.StringVar(&keyFrom1P, "key_from_1p", "", "Read signing key from 1Password ('op://...').")
			fncobra.DurationVar(flags, &tokenDuration, "expiration", time.Hour, "How long the token should be valid for.")
		}).
		DoWithArgs(func(ctx context.Context, args []string) error {
			if partnerID == "" || issuerURL == "" {
				return fnerrors.UsageError("--partner_id or --issuer_url are required", "missing partner account")
			}

			method := jwt.GetSigningMethod(signingMethod)
			if method == nil {
				return fnerrors.UsageError("Supported: ES256, RS256", "unknown signing method %q", method)
			}

			var keyBytes []byte
			if keyFromFile != "" {
				bs, err := os.ReadFile(keyFromFile)
				if err != nil {
					return fnerrors.UsageError("", "failed to read private key: %w", err)
				}
				keyBytes = bs
			} else if keyFrom1P != "" {
				bs, err := read1P(keyFrom1P)
				if err != nil {
					return fnerrors.UsageError("", "failed to invoke `op`: %w", err)
				}
				keyBytes = bs
			} else {
				return fnerrors.UsageError("Either --key_from_file or --key_from_1p is required", "no signing key given")
			}
			key, kid, err := decodePrivateKey(method, keyBytes)
			if err != nil {
				return fnerrors.UsageError("", "failed to decode private key: %w", err)
			}

			issuedAt := time.Now()
			expiresAt := issuedAt.Add(tokenDuration)

			claims := jwt.RegisteredClaims{
				Issuer:    issuerURL,
				Subject:   partnerID,
				Audience:  jwt.ClaimStrings([]string{tokenAudience}),
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				NotBefore: jwt.NewNumericDate(issuedAt),
				IssuedAt:  jwt.NewNumericDate(issuedAt),
			}
			token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
			token.Header["kid"] = kid

			ss, err := token.SignedString(key)
			if err != nil {
				return fmt.Errorf("failed to sign JWT: %w", err)
			}
			fmt.Println(partnerOIDCTokenPrefix + ss)
			return nil
		})
}

func read1P(opID string) ([]byte, error) {
	out, err := exec.Command("op", "read", opID).Output()
	if err != nil {
		return nil, expandExecErr(err)
	}

	// `\n` is added by `op read`.
	return bytes.TrimSuffix(out, []byte{'\n'}), nil
}

func decodePrivateKey(signingMethod jwt.SigningMethod, key []byte) (any, string, error) {
	block, _ := pem.Decode(key)
	if block == nil {
		return nil, "", fmt.Errorf("failed to decode PEM block containing private key")
	}

	switch signingMethod {
	case jwt.SigningMethodES256:
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, "", err
		}

		return key, keyID(&key.PublicKey), nil
	case jwt.SigningMethodRS256:
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, "", err
		}

		return key, keyID(&key.PublicKey), nil
	default:
		return nil, "", fmt.Errorf("signing algorithm %q is not supported", signingMethod.Alg())
	}
}

func keyID(key crypto.PublicKey) string {
	b, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return ""
	}

	kid := sha1.Sum(b)
	return hex.EncodeToString(kid[:])
}

func expandExecErr(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("%w\n%s", err, string(exitErr.Stderr))
	}
	return err
}
