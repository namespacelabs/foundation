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
	p := &provider{}

	combined.RegisterSecretsProvider(func(ctx context.Context, cfg *onepassword.Secret) ([]byte, error) {
		if cfg.SecretReference == "" {
			return nil, fnerrors.BadInputError("invalid 1Password secret configuration: missing field secret_reference")
		}

		return p.Read(ctx, cfg.SecretReference)
	})
}

type provider struct {
	once    sync.Once
	initErr error
}

func (p *provider) Read(ctx context.Context, ref string) ([]byte, error) {
	var data []byte

	p.once.Do(func() {
		// If no account is configured, `op read` does not fail but waits for user input.
		// Hence, we ensure on the first read that a user account is indeed configured.
		if err := ensureAccount(ctx); err != nil {
			p.initErr = err
			return
		}

		// Do the first read serially, so that the user ends up with only one approval popup.
		res, err := read(ctx, ref)
		if err != nil {
			p.initErr = err
			return
		}

		data = res

		// XXX hack!
		// The first read succeeds and unlocks the 1Password client.
		// Still, the second read can fail with `error initializing client: account is not signed in` if issued too quickly :(
		time.Sleep(time.Second)
	})

	// The only writes to p.initErr are inside p.once which is already done at this point.
	if p.initErr != nil {
		return nil, p.initErr
	}

	if data != nil {
		// First read does not need to repeat.
		return data, nil
	}

	return read(ctx, ref)
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

func read(ctx context.Context, ref string) ([]byte, error) {
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
		return data, nil
	})
}
