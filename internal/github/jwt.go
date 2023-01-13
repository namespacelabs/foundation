// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

const (
	userAgent = "actions/oidc-client"
)

func JWT(ctx context.Context, audience string) (string, error) {
	idTokenURL := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_URL")
	if idTokenURL == "" {
		return "", fnerrors.BadDataError("empty ID token request URL")
	}
	idToken := os.Getenv("ACTIONS_ID_TOKEN_REQUEST_TOKEN")
	if idToken == "" {
		return "", fnerrors.BadDataError("empty ID token request token")
	}

	if audience != "" {
		idTokenURL += fmt.Sprintf("&audience=%s", url.QueryEscape(audience))
	}

	req, err := http.NewRequestWithContext(ctx, "GET", idTokenURL, nil)
	if err != nil {
		return "", fnerrors.InvocationError("jwt", "failed to create HTTP request: %w", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Authorization", "Bearer "+idToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fnerrors.InvocationError("jwt", "failed to request github JWT: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fnerrors.InvocationError("jwt", "failed to obtain token: %v", resp.Status)
	}

	var idTokenResponse struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&idTokenResponse); err != nil {
		return "", fnerrors.InvocationError("jwt", "bad response: %w", err)
	}
	if idTokenResponse.Value == "" {
		return "", fnerrors.InvocationError("jwt", "bad response: empty ID token")
	}

	return idTokenResponse.Value, nil
}
