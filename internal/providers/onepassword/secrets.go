// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package onepassword

import (
	"bytes"
	"context"
	"os/exec"

	"namespacelabs.dev/foundation/framework/secrets/combined"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/universe/onepassword"
)

func Register() {
	combined.RegisterSecretsProvider(func(ctx context.Context, cfg *onepassword.Secret) ([]byte, error) {
		if cfg.SecretReference == "" {
			return nil, fnerrors.BadInputError("invalid 1Password secret configuration: missing field secret_reference")
		}

		c := exec.CommandContext(ctx, "op", "read", cfg.SecretReference)

		var b bytes.Buffer
		c.Stdout = &b
		c.Stderr = console.Stderr(ctx)
		c.Stdin = nil
		if err := c.Run(); err != nil {
			return nil, fnerrors.InvocationError("1Password", "failed to invoke %q: %w", c.String(), err)
		}

		return b.Bytes(), nil
	})
}
