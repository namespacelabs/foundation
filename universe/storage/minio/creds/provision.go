// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package creds

import (
	"context"
)

func ProvideCreds(ctx context.Context, _ *CredsRequest, deps ExtensionDeps) (*Creds, error) {
	creds := &Creds{
		User:     string(deps.User.MustValue()),
		Password: string(deps.RootPassword.MustValue()),
	}
	return creds, nil
}
