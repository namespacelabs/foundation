// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubedef

import (
	"strings"

	"namespacelabs.dev/foundation/internal/runtime"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

func DecideKind(obj runtime.Deployable) func(string) runtimepb.ContainerKind {
	return func(containerName string) runtimepb.ContainerKind {
		if obj == nil {
			return runtimepb.ContainerKind_CONTAINER_KIND_UNSPECIFIED
		}

		if ServerCtrName(obj) == containerName {
			return runtimepb.ContainerKind_PRIMARY
		}

		return runtimepb.ContainerKind_SUPPORT
	}
}

func ServerCtrName(obj runtime.Deployable) string {
	return strings.ToLower(obj.GetName()) // k8s doesn't accept uppercase names.
}
