// This file was automatically generated.
package scopes

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
	"namespacelabs.dev/foundation/std/testdata/scopes/data"
)

// Scoped dependencies that are reinstantiated for each call to ProvideScopedData
type ScopedDataDeps struct {
	Data *data.Data
}

type _checkProvideScopedData func(context.Context, fninit.Caller, *Input, *ScopedDataDeps) (*ScopedData, error)

var _ _checkProvideScopedData = ProvideScopedData
