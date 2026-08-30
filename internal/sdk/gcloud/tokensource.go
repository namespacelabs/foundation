// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gcloud

import (
	"context"
	"sync"
	"time"

	"golang.org/x/oauth2"
)

const minimumTokenExpiry = 5 * time.Minute

type TokenSource struct {
	ctx       context.Context
	mu        sync.Mutex
	lastToken *credential
}

var _ oauth2.TokenSource = &TokenSource{}

func NewTokenSource(ctx context.Context) *TokenSource {
	return &TokenSource{ctx: ctx}
}

func (ts *TokenSource) Token() (*oauth2.Token, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	if expired(ts.lastToken) {
		h, err := Credentials(ts.ctx)
		if err != nil {
			return nil, err
		}

		ts.lastToken = h
	}

	return &oauth2.Token{
		AccessToken: ts.lastToken.AccessToken,
		Expiry:      ts.lastToken.TokenExpiry,
	}, nil
}

func expired(creds *credential) bool {
	return creds == nil || time.Now().Add(minimumTokenExpiry).After(creds.TokenExpiry)
}
