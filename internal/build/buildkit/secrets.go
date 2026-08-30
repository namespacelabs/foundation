// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"

	"github.com/moby/buildkit/session/secrets"
)

type secretSource struct {
	base secrets.SecretStore
	m    map[string][]byte
}

func (fs secretSource) GetSecret(ctx context.Context, id string) ([]byte, error) {
	v, ok := fs.m[id]
	if ok {
		return v, nil
	}
	return fs.base.GetSecret(ctx, id)
}
