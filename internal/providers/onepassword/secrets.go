package onepassword

import (
	"bytes"
	"context"
	"fmt"
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

		c := exec.CommandContext(ctx, "op", "read", fmt.Sprintf("%q", cfg.SecretReference))

		var b bytes.Buffer
		c.Stdout = &b
		c.Stderr = console.Stderr(ctx)
		c.Stdin = nil
		if err := c.Run(); err != nil {
			return nil, fnerrors.InternalError("failed to invoke: %w", err)
		}
		return b.Bytes(), nil
	})
}
