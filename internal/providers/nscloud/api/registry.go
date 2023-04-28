// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func RegistryCreds(ctx context.Context) (*authn.Basic, error) {
	token, err := fnapi.FetchTenantToken(ctx)
	if err != nil {
		return nil, err
	}

	return &authn.Basic{
		Username: "token", // XXX: hardcoded as image-registry expects static username.
		Password: token.Raw(),
	}, nil
}
