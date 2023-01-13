// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package clerk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

var (
	ErrUnauthorized = fnerrors.New("Unauthorized")
)

type clerkResponse struct {
	Response struct {
		Identifier string `json:"identifier"`
	} `json:"response"`
	Client struct {
		Sessions []struct {
			User struct {
				ExternalAccounts []struct {
					Provider     string `json:"provider"`
					Username     string `json:"username"`
					Verification struct {
						Status string `json:"status"`
					}
				} `json:"external_accounts"`
			} `json:"user"`
		} `json:"sessions"`
	} `json:"client"`
}

type State struct {
	Email          string `json:"email,omitempty"`
	Name           string `json:"name,omitempty"`
	ClerkClient    string `json:"clerk_client,omitempty"`
	GithubUsername string `json:"github_username,omitempty"`
}

func Login(ctx context.Context, ticket string) (*State, error) {
	form := url.Values{}
	form.Add("strategy", "ticket")
	form.Add("ticket", ticket)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://clerk.namespace.so/v1/client/sign_ins", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Origin", "https://accounts.namespace.so")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("User-Agent", "NamespaceCLI/1.0")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fnerrors.InvocationError("login", "failed to authenticate: %v", resp.Status)
	}

	var client *http.Cookie
	for _, k := range resp.Cookies() {
		if k.Name == "__client" {
			client = k
		}
	}

	if client == nil {
		return nil, fnerrors.InvocationError("login", "missing client cookie")
	}

	var x clerkResponse
	if err := json.NewDecoder(resp.Body).Decode(&x); err != nil {
		return nil, fnerrors.InvocationError("login", "bad response: %w", err)
	}

	state := &State{
		Email:       x.Response.Identifier,
		ClerkClient: client.Value,
	}

	for _, session := range x.Client.Sessions {
		for _, external := range session.User.ExternalAccounts {
			if external.Provider == "oauth_github" && external.Verification.Status == "verified" {
				state.GithubUsername = external.Username
			}
		}
	}

	return state, nil
}

type jwtResponse struct {
	JWT string `json:"jwt"`
}

type cachedJWT struct {
	JWT       string
	CreatedAt time.Time
}

var (
	tokenCache   = map[string]cachedJWT{}
	tokenCacheMu sync.Mutex
)

func JWT(ctx context.Context, st *State) (string, error) {
	tokenCacheMu.Lock()
	defer tokenCacheMu.Unlock()

	if cached, ok := tokenCache[st.ClerkClient]; ok {
		if time.Now().Before(cached.CreatedAt.Add(10 * time.Second)) {
			return cached.JWT, nil
		} else {
			delete(tokenCache, st.ClerkClient)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://clerk.namespace.so/v1/me/tokens", nil)
	if err != nil {
		return "", err
	}

	req.AddCookie(&http.Cookie{
		Name:  "__client",
		Value: st.ClerkClient,
	})

	req.Header.Add("Origin", "https://cli.namespace.so")
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("User-Agent", "NamespaceCLI/1.0")

	now := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fnerrors.InvocationError("jwt", "failed to obtain token: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return "", ErrUnauthorized
		}

		return "", fnerrors.InvocationError("jwt", "failed to obtain token: %v", resp.Status)
	}

	var x jwtResponse
	if err := json.NewDecoder(resp.Body).Decode(&x); err != nil {
		return "", fnerrors.InvocationError("jwt", "bad response: %w", err)
	}

	tokenCache[st.ClerkClient] = cachedJWT{
		CreatedAt: now,
		JWT:       x.JWT,
	}

	return x.JWT, nil
}
