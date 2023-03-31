// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

type TokenSpec struct {
	Version  string `json:"version"`
	TokenURL string `json:"token_url"`
}

func FetchTokenFromSpec(ctx context.Context, specBytes []byte) (*Token, error) {
	var spec TokenSpec
	if err := json.Unmarshal(specBytes, &spec); err != nil {
		return nil, fnerrors.New("token spec is invalid")
	}

	switch spec.Version {
	case "v1":
		resp, err := http.Get(spec.TokenURL)
		if err != nil {
			return nil, fnerrors.New("failed to fetch token: %w", err)
		}

		defer resp.Body.Close()

		tokenBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fnerrors.New("failed to read token: %w", err)
		}

		return &Token{BearerToken: string(tokenBytes)}, nil

	default:
		return nil, fnerrors.New("NSC_TOKEN_SPEC is not supported; only support version=v1")
	}
}
