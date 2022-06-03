// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package registry

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"

	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	"google.golang.org/api/transport"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/providers/gcp"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	c "namespacelabs.dev/foundation/workspace/compute"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/foundation/workspace/tasks"
)

type manager struct {
	devHost  *schema.DevHost
	selector devhost.Selector
}

var _ registry.Manager = manager{}

var DefaultKeychain oci.Keychain = defaultKeychain{}

func Register() {
	registry.Register("gcp/artifactregistry", func(ctx context.Context, env ops.Environment) (m registry.Manager, finalErr error) {
		return manager{devHost: env.DevHost(), selector: devhost.ByEnvironment(env.Proto())}, nil
	})
}

func (em manager) IsInsecure() bool { return false }

func (em manager) Tag(ctx context.Context, repo string, version *provision.BuildID) (oci.AllocatedName, error) {
	return oci.AllocatedName{}, fnerrors.New("unimplemented")
}

func (em manager) AllocateTag(repo string, buildID *provision.BuildID) c.Computable[oci.AllocatedName] {
	return c.Map(tasks.Action("gcp.artifactregistry.alloc-repository"), c.Inputs(), c.Output{NotCacheable: true},
		func(ctx context.Context, r c.Resolved) (oci.AllocatedName, error) {
			return em.Tag(ctx, repo, buildID)
		})
}

func (em manager) AuthRepository(img oci.ImageID) (oci.AllocatedName, error) {
	return oci.AllocatedName{}, fnerrors.New("unimplemented")
}

func (em manager) RefreshAuth(ctx context.Context) ([]*dockertypes.AuthConfig, error) {
	creds, err := transport.Creds(ctx, option.WithScopes(compute.CloudPlatformScope))
	if err != nil {
		return nil, err
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return nil, err
	}

	conf := &gcp.ArtifactRegistryConf{}
	if !em.selector.Select(em.devHost).Get(conf) {
		return nil, nil
	}

	var authcreds []*dockertypes.AuthConfig
	for _, loc := range conf.EnableLocation {
		pkgdev := fmt.Sprintf("%s-docker.pkg.dev", loc)
		authcreds = append(authcreds, &dockertypes.AuthConfig{
			Username:      "oauth2accesstoken",
			Password:      token.AccessToken,
			ServerAddress: pkgdev,
		})
	}

	return authcreds, nil
}

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
