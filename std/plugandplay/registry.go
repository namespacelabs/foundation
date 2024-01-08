// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package plugandplay

import (
	_ "namespacelabs.dev/foundation/internal/artifacts/registry" // For type.googleapis.com/foundation.build.registry.Registry
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/providers/aws/ecr"
	"namespacelabs.dev/foundation/internal/providers/aws/eks"
	"namespacelabs.dev/foundation/internal/providers/gcp/gke"
	artifactregistry "namespacelabs.dev/foundation/internal/providers/gcp/registry"
	_ "namespacelabs.dev/foundation/internal/providers/k3d" // For type.googleapis.com/foundation.providers.k3d.Configuration
	k3dp "namespacelabs.dev/foundation/internal/providers/k3d"
	"namespacelabs.dev/foundation/internal/providers/k3s"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/onepassword"
	"namespacelabs.dev/foundation/internal/providers/teleport"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	_ "namespacelabs.dev/foundation/internal/runtime/kubernetes"
	_ "namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
)

func RegisterProviders() {
	devhost.HasRuntime = runtime.HasRuntime

	ecr.Register()
	eks.Register()
	gke.Register()
	artifactregistry.Register()
	k3dp.Register()
	k3s.Register()
	teleport.Register()
	nscloud.Register()
	onepassword.Register()

	kubernetes.Register()
}
