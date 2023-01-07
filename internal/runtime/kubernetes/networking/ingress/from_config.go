// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package ingress

import (
	"strings"

	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
)

var classes = map[string]kubedef.IngressClass{}

func RegisterIngressClass(name string, class kubedef.IngressClass) {
	classes[name] = class
}

func FromConfig(config *client.Prepared, acceptedClasses []string) (kubedef.IngressClass, error) {
	requestedClass := config.HostEnv.IngressClass
	if requestedClass == "" {
		requestedClass = "nginx"
	}

	if acceptedClasses == nil {
		acceptedClasses = []string{"nginx"}
	}

	if !slices.Contains(acceptedClasses, requestedClass) {
		return nil, fnerrors.BadInputError("ingress class %q is not supported by this cluster type (support: %s)", requestedClass, strings.Join(acceptedClasses, ", "))
	}

	if class, ok := classes[requestedClass]; ok {
		return class, nil
	}

	return nil, fnerrors.BadInputError("ingress class %q is not registered", requestedClass)
}
