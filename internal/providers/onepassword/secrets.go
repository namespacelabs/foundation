// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package onepassword

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"namespacelabs.dev/foundation/framework/secrets/combined"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/universe/onepassword"
)

const cmdTimeout = time.Minute

func Register() {
	combined.RegisterSecretsProvider(func(ctx context.Context, cfg *onepassword.Secret) ([]byte, error) {
		if cfg.SecretReference == "" {
			return nil, fnerrors.BadInputError("invalid 1Password secret configuration: missing field secret_reference")
		}

		readCtx, cancel := context.WithTimeout(ctx, cmdTimeout)
		defer cancel()

		c := exec.CommandContext(readCtx, "op", "read", cfg.SecretReference)

		var b bytes.Buffer
		c.Stdout = &b
		c.Stderr = console.Stderr(ctx)
		if err := c.Run(); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				// If no account is configured, `op read` does not fail but waits for user input.
				// List accounts on timeouts to provide a better error.
				c := exec.CommandContext(readCtx, "op", "account", "list")

				var b bytes.Buffer
				c.Stdout = &b
				c.Stderr = console.Stderr(ctx)
				if err := c.Run(); err != nil {
					return nil, fnerrors.InvocationError("1Password", "failed to invoke %q: %w", c.String(), err)
				}

				if b.String() == "" {
					return nil, fnerrors.InvocationError("1Password", "no 1Password account configured")
				}

				fmt.Fprintf(console.Debug(ctx), "Configured 1Password accounts:\n%s\n", b.String())
			}

			return nil, fnerrors.InvocationError("1Password", "failed to invoke %q: %w", c.String(), err)
		}

		return b.Bytes(), nil
	})
}
