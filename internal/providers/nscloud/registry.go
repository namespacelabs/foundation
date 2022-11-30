// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package nscloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/artifacts/registry"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

var DefaultKeychain oci.Keychain = defaultKeychain{}

const loginEndpoint = "login.namespace.so/token"

type nscloudRegistry struct {
	clusterID string
	registry  *api.ImageRegistry
}

const registryAddr = "registry-fgfo23t6gn9jd834s36g.prod-metal.namespacelabs.nscloud.dev"

func RegisterRegistry() {
	registry.Register("nscloud", func(ctx context.Context, ck cfg.Configuration) (registry.Manager, error) {
		conf, ok := clusterConfigType.CheckGet(ck)
		if !ok || conf.ClusterId == "" {
			return nil, fnerrors.InternalError("missing registry configuration")
		}

		return nscloudRegistry{clusterID: conf.ClusterId}, nil
	})

	oci.RegisterDomainKeychain(registryAddr, DefaultKeychain, oci.Keychain_UseAlways)
}

func (r nscloudRegistry) IsInsecure() bool { return false }

func (r nscloudRegistry) AllocateName(repository string) compute.Computable[oci.AllocatedRepository] {
	return compute.Map(tasks.Action("nscloud.allocate-repository").Arg("repository", repository),
		compute.Inputs().Str("repository", repository).Str("clusterID", r.clusterID),
		compute.Output{},
		func(ctx context.Context, _ compute.Resolved) (oci.AllocatedRepository, error) {
			registry, err := r.fetchRegistry(ctx)
			if err != nil {
				return oci.AllocatedRepository{}, err
			}

			url := registry.EndpointAddress
			if url == "" {
				return oci.AllocatedRepository{}, fnerrors.InternalError("%s: cluster is missing registry", r.clusterID)
			}

			if strings.HasSuffix(url, "/") {
				url += repository
			} else {
				url += "/" + repository
			}

			return oci.AllocatedRepository{
				Parent: r,
				TargetRepository: oci.TargetRepository{
					InsecureRegistry: r.IsInsecure(),
					ImageID:          oci.ImageID{Repository: url},
					// We need to make sure our keychain is attached to the name.
					Keychain: defaultKeychain{},
				},
			}, nil
		})
}

func (r nscloudRegistry) fetchRegistry(ctx context.Context) (*api.ImageRegistry, error) {
	if r.registry != nil {
		return r.registry, nil
	}

	resp, err := api.GetCluster(ctx, r.clusterID)
	if err != nil {
		return nil, err
	}

	return resp.Registry, nil
}

func (r nscloudRegistry) AttachKeychain(imgid oci.ImageID) (oci.AllocatedRepository, error) {
	return registry.AttachStaticKeychain(r, imgid, defaultKeychain{}), nil
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

	tok, err := user.GenerateToken(ctx)
	if err != nil {
		return nil, err
	}

	req.Header.Add("X-Namespace-Token", tok)
	req.Header.Add("Authorization", "Bearer "+tok)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fnerrors.InvocationError("nscloud", "%s: unexpected status when fetching an access token: %d", r, resp.StatusCode)
	}

	tokenData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fnerrors.InvocationError("nscloud", "%s: unexpected error when fetching an access token: %w", r, err)
	}

	var t Token
	if err := json.Unmarshal(tokenData, &t); err != nil {
		return nil, fnerrors.InvocationError("nscloud", "%s: unexpected error when unmarshalling an access token: %w", r, err)
	}

	return &authn.Bearer{Token: t.Token}, nil
}

type Token struct {
	Token string `json:"token"`
}
