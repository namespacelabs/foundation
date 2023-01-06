// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package registry

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	c "namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/sdk/gcloud"
	"namespacelabs.dev/foundation/std/tasks"
)

var DefaultKeychain oci.Keychain = defaultKeychain{}

type defaultKeychain struct{}

func (defaultKeychain) Resolve(ctx context.Context, r authn.Resource) (authn.Authenticator, error) {
	authConfig, err := c.GetValue[authn.AuthConfig](ctx, &obtainAccessToken{})
	if err != nil {
		return nil, err
	}

	return authn.FromConfig(authConfig), nil
}

type obtainAccessToken struct {
	c.DoScoped[authn.AuthConfig]
}

var _ c.Computable[authn.AuthConfig] = &obtainAccessToken{}

func (obtainAccessToken) Action() *tasks.ActionEvent {
	return tasks.Action("gcloud.auth.print-access-token")
}
func (obtainAccessToken) Inputs() *c.In    { return c.Inputs() }
func (obtainAccessToken) Output() c.Output { return c.Output{NotCacheable: true} }
func (obtainAccessToken) Compute(ctx context.Context, _ c.Resolved) (authn.AuthConfig, error) {
	h, err := gcloud.Helper(ctx)
	if err != nil {
		return authn.AuthConfig{}, fnerrors.InvocationError("gcp-artifactregistry", "failed to obtain gcloud access token: %w", err)
	}

	return authn.AuthConfig{
		Username: "oauth2accesstoken",
		Password: h.Credential.AccessToken,
	}, nil
}
