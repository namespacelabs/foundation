// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package creds

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

func ProvideCreds(ctx context.Context, _ fninit.Caller, _ *CredsRequest, deps *CredsDeps) (*Creds, error) {
	creds := &Creds{
		Username: string(deps.User.MustValue()),
		Password: string(deps.Password.MustValue()),
	}
	return creds, nil
}
