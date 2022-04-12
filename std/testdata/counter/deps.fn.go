// This file was automatically generated.
package counter

import (
	"context"

	"namespacelabs.dev/foundation/std/testdata/counter/data"
)

// Scoped dependencies that are reinstantiated for each call to ProvideCounter
type CounterDeps struct {
	Data *data.Data
}

type _checkProvideCounter func(context.Context, *Input, CounterDeps) (*Counter, error)

var _ _checkProvideCounter = ProvideCounter
