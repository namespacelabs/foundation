// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package vault

import (
	"context"
	"fmt"
)

func ProvideClientHandle(ctx context.Context, _ *VaultClientArgs) (*ClientHandle, error) {
	creds, err := ParseCredentialsFromEnv("VAULT_APP_ROLE")
	if err != nil {
		return nil, fmt.Errorf("failed to parse vault credentials: %w", err)
	}

	return creds.ClientHandle(ctx)
}
