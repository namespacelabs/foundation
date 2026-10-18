// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"google.golang.org/protobuf/types/known/anypb"
	internalresources "namespacelabs.dev/foundation/internal/resources"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	stdresources "namespacelabs.dev/foundation/std/resources"
)

// keepProvisionOnlyServers filters server deployables down to the ones required
// to provision resources (e.g. a colocated database). The requested servers are
// not rolled out in provision-only mode.
func keepProvisionOnlyServers(specs []runtime.DeployableSpec, required []schema.PackageName) []runtime.DeployableSpec {
	keep := make(map[schema.PackageName]struct{}, len(required))
	for _, p := range required {
		keep[p] = struct{}{}
	}

	var out []runtime.DeployableSpec
	for _, spec := range specs {
		if spec.PackageRef == nil {
			continue
		}
		if _, ok := keep[spec.PackageRef.AsPackageName()]; ok {
			out = append(out, spec)
		}
	}
	return out
}

// provisionOnlyOutputSink returns a no-op invocation that consumes the given
// resource instance outputs, so the plan's output accounting stays balanced when
// the requested servers (the usual consumers) are not rolled out.
func provisionOnlyOutputSink(instanceIDs []string) (*schema.SerializedInvocation, error) {
	if len(instanceIDs) == 0 {
		return nil, nil
	}

	wrapped, err := anypb.New(&internalresources.OpConsumeResourceOutputs{ResourceInstanceId: instanceIDs})
	if err != nil {
		return nil, err
	}

	after := make([]string, len(instanceIDs))
	for i, id := range instanceIDs {
		after[i] = stdresources.ResourceInstanceCategory(id)
	}

	return &schema.SerializedInvocation{
		Description:    "Consume resource outputs (provision-only)",
		Impl:           wrapped,
		RequiredOutput: instanceIDs,
		Order:          &schema.ScheduleOrder{SchedAfterCategory: after},
	}, nil
}
