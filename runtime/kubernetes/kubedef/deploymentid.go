// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/runtime"
)

func MakeDeploymentId(srv runtime.Deployable) string {
	if srv.GetName() == "" {
		return srv.GetId()
	}

	// k8s doesn't accept uppercase names.
	return fmt.Sprintf("%s-%s", strings.ToLower(srv.GetName()), srv.GetId())
}

func MakeVolumeName(deploymentId, name string) string {
	if (len(deploymentId) + len(name) + 1) > 63 {
		// Deployment id is too long, use an hash instead.
		deploymentId = naming.StableIDN(deploymentId, 8)
	}

	return fmt.Sprintf("%s-%s", deploymentId, name)
}
