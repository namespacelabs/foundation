// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package csrf

import (
	"context"

	"namespacelabs.dev/foundation/std/secrets"
)

type ExtensionDeps struct {
	Token *secrets.Value
}

type _checkPrepare func(context.Context, ExtensionDeps) error

var _ _checkPrepare = Prepare