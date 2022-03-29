// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/schema"
)

func MakeDeploymentId(srv *schema.Server) string {
	if srv.Name == "" {
		return srv.Id
	}

	// k8s doesn't accept uppercase names.
	return fmt.Sprintf("%s-%s", strings.ToLower(srv.Name), srv.Id)
}