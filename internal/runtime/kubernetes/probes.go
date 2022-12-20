// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"k8s.io/apimachinery/pkg/util/intstr"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

type perEnvConf struct {
	dashnessPeriod        int32
	livenessInitialDelay  int32
	readinessInitialDelay int32
	probeTimeout          int32
	failureThreshold      int32
}

var perEnvConfMapping = map[schema.Environment_Purpose]*perEnvConf{
	schema.Environment_DEVELOPMENT: {
		dashnessPeriod:        1,
		livenessInitialDelay:  1,
		readinessInitialDelay: 1,
		probeTimeout:          1,
		failureThreshold:      3,
	},
	schema.Environment_TESTING: {
		dashnessPeriod:        1,
		livenessInitialDelay:  1,
		readinessInitialDelay: 1,
		probeTimeout:          1,
		failureThreshold:      3,
	},
	schema.Environment_PRODUCTION: {
		dashnessPeriod:        3,
		livenessInitialDelay:  1,
		readinessInitialDelay: 3,
		probeTimeout:          1,
		failureThreshold:      5,
	},
}

func toK8sProbe(p *applycorev1.ProbeApplyConfiguration, probevalues *perEnvConf, probe *schema.Probe) (*applycorev1.ProbeApplyConfiguration, error) {
	p = p.WithPeriodSeconds(probevalues.dashnessPeriod).
		WithFailureThreshold(probevalues.failureThreshold).
		WithTimeoutSeconds(probevalues.probeTimeout)

	switch {
	case probe.Http != nil:
		return p.WithHTTPGet(applycorev1.HTTPGetAction().WithPath(probe.Http.GetPath()).
			WithPort(intstr.FromInt(int(probe.Http.GetContainerPort())))), nil

	case probe.Exec != nil:
		return p.WithExec(applycorev1.ExecAction().WithCommand(probe.Exec.Command...)), nil

	default:
		return nil, fnerrors.InternalError("unknown probe type for %q", probe.Kind)
	}
}
