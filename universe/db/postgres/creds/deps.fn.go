// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package creds

import (
	"context"

	"namespacelabs.dev/foundation/std/secrets"
)

type ExtensionDeps struct {
	Password *secrets.Value
	User     *secrets.Value
}

type _checkProvideCreds func(context.Context, string, *CredsRequest, ExtensionDeps) (*Creds, error)

var _ _checkProvideCreds = ProvideCreds