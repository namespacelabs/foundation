// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package registry

import (
	"context"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/sdk/gcloud"
)

var DefaultKeychain oci.Keychain = keychain{}

type keychain struct {
	ts *gcloud.TokenSource
}

func (d keychain) Resolve(ctx context.Context, r authn.Resource) (authn.Authenticator, error) {
	if !strings.HasSuffix(r.RegistryStr(), ".pkg.dev") {
		return nil, nil
	}

	ts := d.ts
	if ts == nil {
		ts = gcloud.NewTokenSource(ctx)
	}

	token, err := ts.Token()
	if err != nil {
		return nil, err
	}

	cfg := authn.AuthConfig{
		Username: "oauth2accesstoken",
		Password: token.AccessToken,
	}

	return authn.FromConfig(cfg), nil
}
