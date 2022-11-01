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
