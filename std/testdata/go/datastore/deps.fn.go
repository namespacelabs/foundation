// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package datastore

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/secrets"
)

type ExtensionDeps struct {
	Cert           *secrets.Value
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, string, *Database, ExtensionDeps) (*DB, error)

var _ _checkProvideDatabase = ProvideDatabase