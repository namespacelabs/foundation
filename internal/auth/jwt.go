// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"crypto/ed25519"
	"embed"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/github"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
)

const (
	tokenTxt         = "token.txt"
	minTokenValidity = 30 * time.Second
)

var (
	// TODO: publish well known jwks and fetch dynamically
	//go:embed ns-jwt.pub
	lib embed.FS

	NamespaceJwtPublicKeyFile string
)

type ed25519PubKey struct {
	ObjectIdentifier struct {
		ObjectIdentifier asn1.ObjectIdentifier
	}
	PublicKey asn1.BitString
}

func publicKey() (ed25519.PublicKey, error) {
	key, err := publicKeyData()
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(key)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing public key")
	}

	var asn1PubKey ed25519PubKey
	if _, err := asn1.Unmarshal(block.Bytes, &asn1PubKey); err != nil {
		return nil, err
	}

	return ed25519.PublicKey(asn1PubKey.PublicKey.Bytes), nil
}

func publicKeyData() ([]byte, error) {
	if NamespaceJwtPublicKeyFile != "" {
		return os.ReadFile(NamespaceJwtPublicKeyFile)
	}

	return fs.ReadFile(lib, "ns-jwt.pub")
}

func fetchTokenForGithub(ctx context.Context, githubTokenExchange func(context.Context, string) (string, error)) (string, error) {
	token, err := loadToken()
	if err != nil {
		return "", err
	}

	var refresh bool
	if token == "" {
		refresh = true
	} else {
		var claims jwt.RegisteredClaims
		tok, err := jwt.ParseWithClaims(token, &claims, func(token *jwt.Token) (interface{}, error) {
			return publicKey()
		})
		if err != nil {
			return "", fnerrors.New("failed to parse JWT: %w", err)
		}

		if !tok.Valid || time.Until(claims.ExpiresAt.Time) < minTokenValidity {
			refresh = true
		}
	}

	if refresh {
		jwt, err := github.JWT(ctx, githubJWTAudience)
		if err != nil {
			return "", err
		}

		token, err = githubTokenExchange(ctx, jwt)
		if err != nil {
			return "", err
		}

		if err := storeToken(token); err != nil {
			return "", err
		}
	}

	return token, nil
}

func storeToken(token string) error {
	configDir, err := dirs.Ensure(dirs.Config())
	if err != nil {
		return err
	}

	if err := os.WriteFile(filepath.Join(configDir, tokenTxt), []byte(token), 0600); err != nil {
		return fnerrors.New("failed to write token data: %w", err)
	}

	return nil
}

func loadToken() (string, error) {
	dir, err := dirs.Config()
	if err != nil {
		return "", err
	}

	p := filepath.Join(dir, tokenTxt)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			// No cached token.
			return "", nil
		}

		return "", err
	}

	return string(data), nil
}
