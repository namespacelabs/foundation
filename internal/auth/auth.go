// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/base64"
	"errors"

	"namespacelabs.dev/foundation/internal/clerk"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func GenerateToken(ctx context.Context) (string, error) {
	userAuth, err := LoadUser()
	if err != nil {
		return "", err
	}

	return GenerateTokenFromUserAuth(ctx, userAuth)
}

func GenerateTokenFromUserAuth(ctx context.Context, userAuth *UserAuth) (string, error) {
	switch {
	case userAuth.Clerk != nil:
		jwt, err := clerk.JWT(ctx, userAuth.Clerk)
		if err != nil {
			if errors.Is(err, clerk.ErrUnauthorized) {
				return "", fnerrors.ReauthError("not logged in")
			}

			return "", err
		}

		return jwt, nil
	case len(userAuth.InternalOpaque) > 0:
		return base64.RawStdEncoding.EncodeToString(userAuth.InternalOpaque), nil
	default:
		return "", fnerrors.ReauthError("not logged in")
	}
}
