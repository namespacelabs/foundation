// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/pkg/browser"
	"github.com/rs/zerolog"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/go-ids"
)

const (
	ApproleAuthMethod = "approle"
	JwtAuthMethod     = "jwt"
	OidcAuthMethod    = "oidc"
)

const (
	VaultJwtAudience = "vault.namespace.systems"

	// Force re-authentication if the token expires in less than this much time.
	ttlBuffer = time.Minute

	oidcCallbackPort = 8250
	oidcLoginTimeout = 5 * time.Minute
)

type AuthMethod string

type Provider struct {
	creds *Credentials
	opts  []vault.ClientOption

	m    sync.Mutex
	conn *ClientHandle
}

func ProviderFromEnv(key string, options ...vault.ClientOption) (*Provider, error) {
	if os.Getenv(key) == "" {
		return nil, fmt.Errorf("vault: environment variable %q not set", key)
	}

	creds, err := ParseCredentialsFromEnv(key)
	if err != nil {
		return nil, fmt.Errorf("vault: environment variable %q could not be parsed: %w", key, err)
	}

	return NewProvider(creds, options...)
}

func NewProvider(creds *Credentials, opts ...vault.ClientOption) (*Provider, error) {
	return &Provider{creds: creds, opts: opts}, nil
}

func (p *Provider) Get(ctx context.Context) (*vault.Client, error) {
	conn, err := p.connect(ctx)
	if err != nil {
		return nil, err
	}

	return conn.Get(ctx)
}

func (p *Provider) connect(ctx context.Context) (*ClientHandle, error) {
	p.m.Lock()
	defer p.m.Unlock()

	if p.conn == nil {
		conn, err := p.creds.ClientHandle(ctx, p.opts...)
		if err != nil {
			return nil, err
		}
		p.conn = conn
	}

	return p.conn, nil
}

type ClientHandle struct {
	creds  *Credentials
	client *vault.Client
	auth   *vault.ResponseAuth
	leased time.Time

	m sync.Mutex
}

func (h *ClientHandle) Get(ctx context.Context) (*vault.Client, error) {
	h.m.Lock()
	defer h.m.Unlock()

	if time.Until(h.expires()) > ttlBuffer {
		return h.client, nil
	}

	if err := h.renew(ctx); err != nil {
		return nil, err
	}

	return h.client, nil
}

func (h *ClientHandle) authenticate(ctx context.Context) error {
	// Always prefer env var.
	if t := os.Getenv("VAULT_TOKEN"); t != "" {
		zerolog.Ctx(ctx).Debug().Msg("use VAULT_TOKEN environment variable")
		return h.client.SetToken(t)
	}

	var vaultAuth *vault.ResponseAuth
	switch h.creds.AuthMethod {
	case JwtAuthMethod:
		resp, err := JwtLogin(ctx, h.client, h.creds.AuthMount, h.creds.JwtAudience)
		if err != nil {
			return err
		}

		vaultAuth = resp
	case OidcAuthMethod:
		resp, err := OidcLogin(ctx, h.client, h.creds.AuthMount)
		if err != nil {
			return err
		}

		vaultAuth = resp
	// For backward compatibility, by default assume that approle method is used.
	case ApproleAuthMethod:
		fallthrough
	default:
		var opts []vault.RequestOption
		if h.creds.AuthMount != "" {
			opts = append(opts, vault.WithMountPath(h.creds.AuthMount))
		}
		resp, err := h.client.Auth.AppRoleLogin(ctx, schema.AppRoleLoginRequest{
			RoleId:   h.creds.RoleId,
			SecretId: h.creds.SecretId,
		}, opts...)
		if err != nil {
			return err
		}

		vaultAuth = resp.Auth
	}

	h.auth = vaultAuth
	h.leased = time.Now()

	zerolog.Ctx(ctx).Debug().Dur("lease_duration", h.ttl()).Msg("vault: authenticated")
	return h.client.SetToken(vaultAuth.ClientToken)
}

func (h *ClientHandle) renew(ctx context.Context) error {
	if h.auth == nil || !h.auth.Renewable {
		return h.authenticate(ctx)
	}

	res, err := h.client.Auth.TokenRenewSelf(ctx, schema.TokenRenewSelfRequest{})
	if err != nil {
		// The Vault client library already handles retries, so if renewing the
		// token fails, we assume it can no longer be renewed. This can happen if
		// the token was revoked, or if it reached its maximum TTL.
		zerolog.Ctx(ctx).Warn().Msg("vault: token renewal failed, forcing re-auth")
		return h.authenticate(ctx)
	}

	h.auth = res.Auth
	zerolog.Ctx(ctx).Debug().Dur("lease_duration", h.ttl()).Msg("vault: token renewed")
	h.leased = time.Now()
	return nil
}

func (h *ClientHandle) ttl() time.Duration {
	if h.auth == nil {
		return 0
	}

	return time.Duration(h.auth.LeaseDuration) * time.Second
}

func (h *ClientHandle) expires() time.Time {
	if h.auth == nil {
		return time.Time{}
	}

	return h.leased.Add(h.ttl())
}

type Credentials struct {
	AuthMethod AuthMethod `json:"auth_method,omitempty"`
	AuthMount  string     `json:"auth_mount,omitempty"`

	JwtAudience string `json:"jwt_audience,omitempty"`

	RoleId   string `json:"role_id,omitempty"`
	SecretId string `json:"secret_id,omitempty"`

	VaultAddress   string `json:"vault_address,omitempty"`
	VaultNamespace string `json:"vault_namespace,omitempty"`
}

func (c *Credentials) Encode() ([]byte, error) {
	return json.Marshal(c)
}

func (c *Credentials) ClientHandle(ctx context.Context, options ...vault.ClientOption) (*ClientHandle, error) {
	client, err := vault.New(append([]vault.ClientOption{
		vault.WithAddress(c.VaultAddress),
	}, options...)...)
	if err != nil {
		return nil, err
	}

	if c.VaultNamespace != "" {
		if err := client.SetNamespace(c.VaultNamespace); err != nil {
			return nil, err
		}
	}

	return &ClientHandle{
		creds:  c,
		client: client,
	}, nil
}

func ParseCredentials(data []byte) (*Credentials, error) {
	c := Credentials{}
	return &c, json.Unmarshal(data, &c)
}

func ParseCredentialsFromEnv(key string) (*Credentials, error) {
	return ParseCredentials([]byte(os.Getenv(key)))
}

func OidcLogin(ctx context.Context, client *vault.Client, authMount string) (*vault.ResponseAuth, error) {
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

	var opts []vault.RequestOption
	if authMount != "" {
		opts = append(opts, vault.WithMountPath(authMount))
	}

	r, err := client.Auth.JwtOidcRequestAuthorizationUrl(ctx, schema.JwtOidcRequestAuthorizationUrlRequest{
		ClientNonce: clientNonce,
		RedirectUri: fmt.Sprintf("http://localhost:%d/oidc/callback", oidcCallbackPort),
	}, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to request OIDC authorization URL: %w", err)
	}

	authUrl, ok := r.Data["auth_url"].(string)
	if !ok || authUrl == "" {
		return nil, errors.New("returned invalid OIDC authorization URL")
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
			r, err = client.Auth.JwtOidcCallback(ctx, clientNonce, resp.code, resp.state, opts...)
			if err != nil {
				return nil, fmt.Errorf("failed to login using OIDC provider: %w", err)
			}

			return r.Auth, nil
		case <-ctx.Done():
			return nil, fmt.Errorf("OIDC login did not complete on time: %w", ctx.Err())
		}
	}
}

func JwtLogin(ctx context.Context, client *vault.Client, authMount, audience string) (*vault.ResponseAuth, error) {
	var aud = audience
	if aud == "" {
		aud = VaultJwtAudience
	}

	idTokenResp, err := fnapi.IssueIdToken(ctx, audience, 1)
	if err != nil {
		return nil, err
	}

	var opts []vault.RequestOption
	if authMount != "" {
		opts = append(opts, vault.WithMountPath(authMount))
	}

	loginResp, err := client.Auth.JwtLogin(ctx, schema.JwtLoginRequest{Jwt: idTokenResp.IdToken}, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to login to vault: %w", err)
	}

	return loginResp.Auth, nil
}

func AppRoleLogin(ctx context.Context) {

}
