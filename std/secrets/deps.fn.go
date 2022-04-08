// This file was automatically generated.
package secrets

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
)

type _checkProvideSecret func(context.Context, fninit.Caller, *Secret) (*Value, error)

var _ _checkProvideSecret = ProvideSecret
