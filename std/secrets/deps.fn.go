// This file was automatically generated.
package secrets

import (
	"context"
)

type _checkProvideSecret func(context.Context, string, *Secret) (*Value, error)

var _ _checkProvideSecret = ProvideSecret
