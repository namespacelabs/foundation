// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"strings"
)

func MakeDeploymentId(srv Deployable) string {
	if srv.GetName() == "" {
		return srv.GetId()
	}

	// k8s doesn't accept uppercase names.
	return fmt.Sprintf("%s-%s", strings.ToLower(srv.GetName()), srv.GetId())
}
