// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package teleport

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	dockertypes "github.com/docker/cli/cli/config/types"
	"github.com/google/go-containerregistry/pkg/authn"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/providers/aws/ecr"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/teleport/configuration"
)

type ecrTeleportKeychain struct {
	conf *configuration.Configuration
}

func (tk ecrTeleportKeychain) Resolve(ctx context.Context, res authn.Resource) (authn.Authenticator, error) {
	if tk.conf.GetTeleport().GetEcrCredentialsProxyApp() == "" {
		return authn.DefaultKeychain.Resolve(res)
	}

	config, err := tk.ecrAuth(ctx)
	if err != nil {
		return nil, err
	}

	return authn.FromConfig(authn.AuthConfig{
		Username: config.Username,
		Password: config.Password,
	}), nil
}

func (tk ecrTeleportKeychain) ecrAuth(ctx context.Context) (*dockertypes.AuthConfig, error) {
	appCreds, err := resolveTeleportAppCreds(tk.conf.GetTeleport(), tk.conf.GetTeleport().GetEcrCredentialsProxyApp())
	if err != nil {
		return nil, err
	}

	return tasks.Return(ctx, tasks.Action("teleport.ecr-auth"), func(ctx context.Context) (*dockertypes.AuthConfig, error) {
		return ecr.RefreshAuth(ctx,
			func(ctx context.Context) ([]types.AuthorizationData, error) {
				cert, err := tls.LoadX509KeyPair(appCreds.certFile, appCreds.keyFile)
				if err != nil {
					return nil, fnerrors.New("teleport: failed to load client TLS certificate")
				}

				httpClient := &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: &tls.Config{
							Certificates: []tls.Certificate{cert},
						},
					},
				}

				req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://%s", appCreds.endpoint), nil)
				if err != nil {
					fmt.Fprintf(console.Debug(ctx), "teleport: failed to create ECR credentials request: %v\n", err)
					return nil, fnerrors.New("teleport: failed to create ECR credentials request")
				}

				resp, err := httpClient.Do(req)
				if err != nil {
					fmt.Fprintf(console.Debug(ctx), "HTTP request to Teleport app failed: %v\n", err)
					return nil, fnerrors.New("teleport: failed to request ECR credentials")
				}
				defer resp.Body.Close()

				data, err := io.ReadAll(resp.Body)
				if err != nil {
					fmt.Fprintf(console.Debug(ctx), "failed to read HTTP request body: %v\n", err)
					return nil, fnerrors.New("teleport: failed to read ECR credentials")
				}

				var authzData []types.AuthorizationData
				if err := json.Unmarshal(data, &authzData); err != nil {
					fmt.Fprintf(console.Debug(ctx), "failed to parse HTTP request body: %v\n", err)
					return nil, fnerrors.New("teleport: failed to parse ECR credentials")
				}

				return authzData, nil
			},
			func(ctx context.Context) (string, error) {
				return tk.conf.GetRegistry(), nil
			},
		)
	})
}
