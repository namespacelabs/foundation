// This file was automatically generated.
package scopes

import (
	"context"

	"namespacelabs.dev/foundation/std/testdata/scopes/data"
)

// Scoped dependencies that are reinstantiated for each call to ProvideScopedData
type ScopedDataDeps struct {
	Data *data.Data
}

type _checkProvideScopedData func(context.Context, *Input, *ScopedDataDeps) (*ScopedData, error)

var _ _checkProvideScopedData = ProvideScopedData
