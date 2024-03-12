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
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/foundation/universe/onepassword"
)

const cmdTimeout = time.Minute

func Register() {
	p := provider{
		reader: &cachedReader{
			cache: map[string][]byte{},
		},
	}

	p.cond = sync.NewCond(&p.mu)

	combined.RegisterSecretsProvider(func(ctx context.Context, cfg *onepassword.Secret) ([]byte, error) {
		if cfg.SecretReference == "" {
			return nil, fnerrors.BadInputError("invalid 1Password secret configuration: missing field secret_reference")
		}

		return p.Read(ctx, cfg.SecretReference)
	})
}

type provider struct {
	mu          sync.Mutex
	cond        *sync.Cond
	waiting     int // The first waiter, will also check once if there is an account and acquire human approval.
	initialized bool
	initErr     error

	reader *cachedReader
}

func (p *provider) Read(ctx context.Context, ref string) ([]byte, error) {
	p.mu.Lock()

	rev := p.waiting
	p.waiting++

	if rev > 0 {
		if p.initErr != nil {
			defer p.mu.Unlock()
			return nil, p.initErr
		}

		if !p.initialized {
			p.cond.Wait()

			if p.initErr != nil {
				defer p.mu.Unlock()
				return nil, p.initErr
			}
		}

		// Do not defer here, so that approved reads can go in parallel.
		p.mu.Unlock()
		return p.reader.read(ctx, ref)
	}

	p.mu.Unlock()

	// If no account is configured, `op read` does not fail but waits for user input.
	// Hence, we ensure on the first read that a user account is indeed configured.
	if err := ensureAccount(ctx); err != nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		defer p.cond.Broadcast()

		p.initErr = err
		return nil, err
	}

	// Do the first read serially, so that the user ends up with only one approval popup.
	data, err := p.reader.read(ctx, ref)
	if err != nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		defer p.cond.Broadcast()

		p.initErr = err
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.cond.Broadcast()

	// Set this bit so that no future read needs to wait.
	p.initialized = true
	return data, nil
}

func ensureAccount(ctx context.Context) error {
	return tasks.Return0(ctx, tasks.Action("1password.ensure"), func(ctx context.Context) error {
		if os.Getenv("OP_SERVICE_ACCOUNT_TOKEN") != "" {
			return nil
		}

		// Handle manual logins.
		c := exec.CommandContext(ctx, "op", "account", "list")

		var b bytes.Buffer
		c.Stdout = &b
		c.Stderr = console.Output(ctx, "1Password")
		if err := c.Run(); err != nil {
			return fnerrors.InvocationError("1Password", "failed to invoke %q: %w", c.String(), err)
		} else if b.String() == "" {
			return fnerrors.InvocationError("1Password", "no 1Password account configured")
		}

		fmt.Fprintf(console.Debug(ctx), "Configured 1Password accounts:\n%s\n", b.String())
		return nil
	})
}

type cachedReader struct {
	mu    sync.Mutex
	cache map[string][]byte
}

func (cr *cachedReader) read(ctx context.Context, ref string) ([]byte, error) {
	cr.mu.Lock()
	if data, ok := cr.cache[ref]; ok {
		defer cr.mu.Unlock()
		return data, nil
	}
	cr.mu.Unlock()

	return tasks.Return(ctx, tasks.Action("1password.read").Arg("ref", ref), func(ctx context.Context) ([]byte, error) {
		c := exec.CommandContext(ctx, "op", "read", ref)

		var b bytes.Buffer
		c.Stdout = &b
		c.Stderr = console.Output(ctx, "1Password")
		if err := c.Run(); err != nil {
			return nil, fnerrors.InvocationError("1Password", "failed to invoke %q: %w", c.String(), err)
		}

		// `\n` is added by `op read`.
		data := bytes.TrimSuffix(b.Bytes(), []byte{'\n'})

		cr.mu.Lock()
		defer cr.mu.Unlock()

		cr.cache[ref] = data
		return data, nil
	})
}
