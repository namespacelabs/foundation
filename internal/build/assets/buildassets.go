// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package assets

import (
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
)

type AvailableBuildAssets struct {
	IngressFragments compute.Computable[[]*schema.IngressFragment]
}
