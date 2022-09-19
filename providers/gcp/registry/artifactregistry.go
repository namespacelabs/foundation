// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package registry

import (
	"bytes"
	"context"
	"os/exec"

	"github.com/google/go-containerregistry/pkg/authn"
	c "namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/workspace/tasks"
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
	var out bytes.Buffer
	cmd := exec.CommandContext(ctx, "gcloud", "auth", "print-access-token")
	cmd.Stdout = &out
	cmd.Stderr = console.TypedOutput(ctx, "gcloud", console.CatOutputTool)
	if err := cmd.Run(); err != nil {
		return authn.AuthConfig{}, fnerrors.InvocationError("failed to obtain gcloud access token: %w", err)
	}

	return authn.AuthConfig{
		Username: "oauth2accesstoken",
		Password: out.String(),
	}, nil
}
