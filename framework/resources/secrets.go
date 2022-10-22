// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package resources

import (
	"fmt"
	"os"

	"namespacelabs.dev/foundation/library/runtime"
)

func ReadSecret(r *Parsed, resource string) ([]byte, error) {
	secret := &runtime.SecretInstance{}
	if err := r.Unmarshal(resource, secret); err != nil {
		return nil, err
	}

	if secret.Path == "" {
		return nil, fmt.Errorf("secret %s is missing a path to read from", resource)
	}

	data, err := os.ReadFile(secret.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret %s from path %s: %w", resource, secret.Path, err)
	}

	return data, nil
}
