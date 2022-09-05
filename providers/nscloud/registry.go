// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nscloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	reg "namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/compute"
)

var DefaultKeychain oci.Keychain = defaultKeychain{}

const loginEndpoint = "login.namespace.so/token"

type r struct{}

func RegisterRegistry() {
	reg.Register("nscloud", func(ctx context.Context, ck planning.Configuration) (reg.Manager, error) {
		return r{}, nil
	})

	oci.RegisterDomainKeychain(registryAddr, DefaultKeychain, oci.Keychain_UseAlways)
}

func (r) IsInsecure() bool { return false }

func (r) AllocateName(repository string) compute.Computable[oci.AllocatedName] {
	w := registryAddr

	if strings.HasSuffix(w, "/") {
		w += repository
	} else {
		w += "/" + repository
	}

	imgid := oci.ImageID{Repository: w}

	return reg.StaticName(&registry.Registry{Url: registryAddr}, imgid, DefaultKeychain)
}

func (r) AuthRepository(imgid oci.ImageID) (oci.AllocatedName, error) {
	return oci.AllocatedName{ImageID: imgid, Keychain: DefaultKeychain}, nil
}

type defaultKeychain struct{}

func (dk defaultKeychain) Resolve(ctx context.Context, r authn.Resource) (authn.Authenticator, error) {
	user, err := fnapi.LoadUser()
	if err != nil {
		return nil, err
	}

	ref, err := name.ParseReference(r.String())
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	values.Add("scope", fmt.Sprintf("repository:%s:push,pull", ref.Context().RepositoryStr()))
	values.Add("service", "Authentication")

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s?%s", loginEndpoint, values.Encode()), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-Namespace-Token", base64.RawStdEncoding.EncodeToString(user.Opaque))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fnerrors.InvocationError("%s: unexpected status when fetching an access token: %d", r, resp.StatusCode)
	}

	tokenData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fnerrors.InvocationError("%s: unexpected error when fetching an access token: %w", r, err)
	}

	var t Token
	if err := json.Unmarshal(tokenData, &t); err != nil {
		return nil, fnerrors.InvocationError("%s: unexpected error when unmarshalling an access token: %w", r, err)
	}

	return &authn.Bearer{Token: t.Token}, nil
}

type Token struct {
	Token string `json:"token"`
}
