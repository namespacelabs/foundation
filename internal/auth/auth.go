// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"namespacelabs.dev/foundation/internal/clerk"
)

func GenerateToken(ctx context.Context) (string, error) {
	userAuth, err := LoadUser()
	if err != nil {
		return "", err
	}

	if userAuth.Clerk != nil {
		jwt, err := clerk.JWT(ctx, userAuth.Clerk)
		if err != nil {
			if errors.Is(err, clerk.ErrUnauthorized) {
				return "", ErrRelogin
			}
			return "", err
		}
		return fmt.Sprintf("jwt:%s", jwt), nil
	}
	return base64.RawStdEncoding.EncodeToString(userAuth.InternalOpaque), nil
}
