// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package assets

import (
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
)

type AvailableBuildAssets struct {
	IngressFragments compute.Computable[[]*schema.IngressFragment]
}
