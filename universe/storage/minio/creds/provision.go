// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package creds

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/framework/resources"
)

const (
	userResourceRef     = "namespacelabs.dev/foundation/universe/storage/minio/creds:root-user"
	passwordResourceRef = "namespacelabs.dev/foundation/universe/storage/minio/creds:root-password"
)

func ProvideCreds(ctx context.Context, _ *CredsRequest) (*Creds, error) {
	rs, err := resources.LoadResources()
	if err != nil {
		return nil, err
	}

	user, err := resources.ReadSecret(rs, userResourceRef)
	if err != nil {
		return nil, fmt.Errorf("failed to read Minio user: %w", err)
	}

	password, err := resources.ReadSecret(rs, passwordResourceRef)
	if err != nil {
		return nil, fmt.Errorf("failed to read Minio password: %w", err)
	}

	return &Creds{
		User:     string(user),
		Password: string(password),
	}, nil
}
