// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"strings"

	"namespacelabs.dev/foundation/runtime"
	schema "namespacelabs.dev/foundation/schema"
)

func MakeDeploymentId(srv runtime.Deployable) string {
	if srv.GetName() == "" {
		return srv.GetId()
	}

	// k8s doesn't accept uppercase names.
	return fmt.Sprintf("%s-%s", strings.ToLower(srv.GetName()), srv.GetId())
}

func MakeVolumeName(v *schema.Volume) string {
	if v.Inline {
		return LabelLike("vi", v.Name)
	}

	return LabelLike("v", v.Name)
}

func MakeResourceName(deploymentId string, suffix ...string) string {
	return DomainFragLike(append([]string{deploymentId}, suffix...)...)
}
