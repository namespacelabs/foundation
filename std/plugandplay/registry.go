// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package plugandplay

import (
	"namespacelabs.dev/foundation/internal/providers/aws/ecr"
	"namespacelabs.dev/foundation/internal/providers/aws/eks"
	"namespacelabs.dev/foundation/internal/providers/gcp/gke"
	artifactregistry "namespacelabs.dev/foundation/internal/providers/gcp/registry"
	k3dp "namespacelabs.dev/foundation/internal/providers/k3d"
	"namespacelabs.dev/foundation/internal/providers/k3s"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/onepassword"
)

func RegisterConfigurationProvider() {
	ecr.Register()
	eks.Register()
	gke.Register()
	artifactregistry.Register()
	k3dp.Register()
	k3s.Register()
	nscloud.Register()
	onepassword.Register()
}
