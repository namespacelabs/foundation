// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

// This file was automatically generated.
package secrets

import (
	"context"
)

type _checkProvideSecret func(context.Context, string, *Secret) (*Value, error)

var _ _checkProvideSecret = ProvideSecret