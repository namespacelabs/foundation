// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	vaultclient "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/pkg/browser"
	"namespacelabs.dev/foundation/framework/secrets/combined"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/tcache"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/vault"
	"namespacelabs.dev/go-ids"
)

const (
	vaulTokenEnvKey  = "VAULT_TOKEN"
	vaultJwtAudience = "vault.namespace.systems"

	oidcCallbackPort = 8250
	oidcLoginTimeout = 5 * time.Minute
)

var (
	vaultConfigType       = cfg.DefineConfigType[*vault.VaultProvider]()
	certificateConfigType = cfg.DefineConfigType[*vault.CertificateConfig]()

	clients = tcache.NewCache[*vaultclient.Client]()
)

func GetVaultConfig(cfg cfg.Configuration) (*vault.VaultProvider, bool) {
	return vaultConfigType.CheckGet(cfg)
}

func GetCertificateConfig(cfg cfg.Configuration) (*vault.CertificateConfig, bool) {
	return certificateConfigType.CheckGet(cfg)
}

func Register() {
	combined.RegisterSecretsProvider(appRoleProvider)
	combined.RegisterSecretsProvider(certificateProvider)
	combined.RegisterSecretsProvider(certificateAuthorityProvider)
	combined.RegisterSecretsProvider(secretProvider)
}

func login(ctx context.Context, vaultCfg *vault.VaultProvider) (*vaultclient.Client, error) {
	key := vaultCfg.GetAddress() + vaultCfg.GetNamespace() + vaultCfg.GetAuthMount()
	return clients.Compute(key, func() (*vaultclient.Client, error) {
		return tasks.Return(ctx, tasks.Action("vault.login").Arg("namespace", vaultCfg.GetNamespace()).Arg("address", vaultCfg.GetAddress()),
			func(ctx context.Context) (*vaultclient.Client, error) {
				client, err := vaultclient.New(
					vaultclient.WithAddress(vaultCfg.GetAddress()),
					vaultclient.WithRequestTimeout(vaultRequestTimeout),
					withIssue257Workaround(),
				)
				if err != nil {
					return nil, fnerrors.InvocationError("vault", "failed to create vault client: %w", err)
				}

				client.SetNamespace(vaultCfg.GetNamespace())

				// Vault by default always prefers a token set in VAULT_TOKEN env var. We do the same.
				// Useful in case of VAULT_TOKEN provided by the 3rd party (e.g. by CI, etc).
				if token := os.Getenv(vaulTokenEnvKey); token != "" {
					client.SetToken(token)
					return client, nil
				}

				var vaultToken string
				switch vaultCfg.GetAuthMethod() {
				case "jwt":
					token, err := jwtLogin(ctx, client, vaultCfg, vaultJwtAudience)
					if err != nil {
						return nil, err
					}

					vaultToken = token
				case "oidc":
					token, err := oidcLogin(ctx, client, vaultCfg)
					if err != nil {
						return nil, err
					}

					vaultToken = token
				default:
					return nil, fnerrors.BadDataError("unknown authentication method %q; valid methods: jwt, oidc", vaultCfg.AuthMethod)
				}

				client.SetToken(vaultToken)
				return client, nil
			},
		)
	})
}

func jwtLogin(ctx context.Context, client *vaultclient.Client, vaultCfg *vault.VaultProvider, audience string) (string, error) {
	idTokenResp, err := fnapi.IssueIdToken(ctx, audience, 1)
	if err != nil {
		return "", err
	}

	loginResp, err := client.Auth.JwtLogin(ctx, schema.JwtLoginRequest{Jwt: idTokenResp.IdToken},
		vaultclient.WithMountPath(vaultCfg.GetAuthMount()),
	)
	if err != nil {
		return "", fnerrors.InvocationError("vault", "failed to login to vault: %w", err)
	}

	if loginResp.Auth == nil {
		return "", fnerrors.InvocationError("vault", "missing vault login auth data: %w", err)
	}
	return loginResp.Auth.ClientToken, nil

}

func oidcLogin(ctx context.Context, client *vaultclient.Client, vaultCfg *vault.VaultProvider) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, oidcLoginTimeout)
	defer cancel()

	type callbackResponse struct {
		code, state string
	}

	callbackCh := make(chan callbackResponse, 1)
	callbackServer := &http.Server{
		Addr: fmt.Sprintf("127.0.0.1:%d", oidcCallbackPort),
	}
	http.HandleFunc("/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "Login is sucessful! This page can be closed now.")
		callbackCh <- callbackResponse{
			code:  r.URL.Query().Get("code"),
			state: r.URL.Query().Get("state"),
		}
	})
	go func() {
		if err := callbackServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(console.Debug(ctx), "failed to start OIDC callback server: %v", err)
		}
	}()
	defer callbackServer.Shutdown(ctx)

	clientNonce := ids.NewRandomBase32ID(20)
	r, err := client.Auth.JwtOidcRequestAuthorizationUrl(ctx,
		schema.JwtOidcRequestAuthorizationUrlRequest{
			ClientNonce: clientNonce,
			RedirectUri: fmt.Sprintf("http://localhost:%d/oidc/callback", oidcCallbackPort),
		},
		vaultclient.WithMountPath(vaultCfg.GetAuthMount()),
	)
	if err != nil {
		return "", fnerrors.InvocationError("vault", "failed to request OIDC authorization URL: %v", err)
	}

	authUrl, ok := r.Data["auth_url"].(string)
	if !ok || authUrl == "" {
		return "", fnerrors.InvocationError("vault", "returned invalid OIDC authorization URL")
	}

	fmt.Fprintf(console.Stdout(ctx), "Complete the login via your OIDC provider. Launching browser to:\n\n")
	fmt.Fprintf(console.Stdout(ctx), "\t%s\n\n", authUrl)
	if err := browser.OpenURL(authUrl); err != nil {
		fmt.Fprintf(console.Debug(ctx), "failed to open browser: %v\n", err)
	}

	fmt.Fprintf(console.Stdout(ctx), "Waiting for OIDC authentication to complete...\n")
	for {
		select {
		case resp := <-callbackCh:
			r, err = client.Auth.JwtOidcCallback(ctx, clientNonce,
				resp.code, resp.state,
				vaultclient.WithMountPath(vaultCfg.GetAuthMount()),
			)
			if err != nil {
				return "", fnerrors.InvocationError("vault", "failed to login using OIDC provider: %v", err)
			}

			return r.Auth.ClientToken, nil
		case <-ctx.Done():
			return "", fnerrors.InvocationError("vault", "OIDC login did not complete on time: %v", ctx.Err())
		}
	}
}
