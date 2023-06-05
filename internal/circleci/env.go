// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package circleci

import (
	"os"

	"namespacelabs.dev/foundation/internal/fnerrors"
)

func IsRunningInCircleci() bool {
	return os.Getenv("CIRCLECI") == "true"
}

func GetOidcTokenV2() (string, error) {
	if token, ok := os.LookupEnv("CIRCLE_OIDC_TOKEN_V2"); ok {
		return token, nil
	}

	return "", fnerrors.BadDataError("no CircleCI OIDC token found")
}
