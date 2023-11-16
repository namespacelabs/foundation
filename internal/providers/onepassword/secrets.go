// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package onepassword

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"namespacelabs.dev/foundation/framework/secrets/combined"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/universe/onepassword"
)

const cmdTimeout = time.Minute

func Register() {
	var p provider

	combined.RegisterSecretsProvider(func(ctx context.Context, cfg *onepassword.Secret) ([]byte, error) {
		if cfg.SecretReference == "" {
			return nil, fnerrors.BadInputError("invalid 1Password secret configuration: missing field secret_reference")
		}

		return p.Read(ctx, cfg.SecretReference)
	})
}

type provider struct {
	mu               sync.Mutex
	ensureAccountErr error
	once             sync.Once
}

func (p *provider) Read(ctx context.Context, ref string) ([]byte, error) {
	// Serialize reads to ensure there is only one approval pop-up.
	p.mu.Lock()
	defer p.mu.Unlock()

	// If no account is configured, `op read` does not fail but waits for user input.
	// Hence, we ensure that a user account is indeed configured.
	if err := p.ensureAccount(ctx); err != nil {
		return nil, err
	}

	c := exec.CommandContext(ctx, "op", "read", ref)

	var b bytes.Buffer
	c.Stdout = &b
	c.Stderr = console.Stderr(ctx)
	if err := c.Run(); err != nil {

		return nil, fnerrors.InvocationError("1Password", "failed to invoke %q: %w", c.String(), err)
	}

	return b.Bytes(), nil
}

func (p *provider) ensureAccount(ctx context.Context) error {
	// Only check once if there is an account.
	p.once.Do(func() {
		if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") != "" {
			return
		}

		// Handle manual logins.
		c := exec.CommandContext(ctx, "op", "account", "list")

		var b bytes.Buffer
		c.Stdout = &b
		c.Stderr = console.Stderr(ctx)
		if err := c.Run(); err != nil {
			p.ensureAccountErr = fnerrors.InvocationError("1Password", "failed to invoke %q: %w", c.String(), err)
		} else if b.String() == "" {
			p.ensureAccountErr = fnerrors.InvocationError("1Password", "no 1Password account configured")
		}

		fmt.Fprintf(console.Debug(ctx), "Configured 1Password accounts:\n%s\n", b.String())
	})

	return p.ensureAccountErr
}
