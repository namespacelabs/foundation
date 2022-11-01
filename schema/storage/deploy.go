// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package storage

import (
	"namespacelabs.dev/foundation/internal/planning/constants"
)

// Heuristic
func (p *NetworkPlan) IsDeploymentFinished() bool {
	hasIngress := false
	for _, endpoint := range p.Endpoints {
		if endpoint.LocalPort == 0 {
			return false
		}
		hasIngress = hasIngress || endpoint.IsIngress()
	}

	return hasIngress
}

func (e *Endpoint) IsIngress() bool {
	return e != nil && e.EndpointOwner == "" && e.ServiceName == constants.IngressServiceName
}
