// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeops

import (
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/networking/ingress"
)

var OutputKubeApiURLs = false

func Register() {
	registerApply()
	registerCreate()
	RegisterCreateSecret()
	registerDelete()
	registerDeleteList()
	registerCleanup()
	registerApplyRoleBinding()
	registerEnsureRuntimeConfig()
	registerEnsureDeployment()

	ingress.Register()
}
