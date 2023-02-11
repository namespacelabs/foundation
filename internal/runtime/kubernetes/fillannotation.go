// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubernetes

import (
	"context"

	"namespacelabs.dev/foundation/framework/resources"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	runtimepb "namespacelabs.dev/foundation/schema/runtime"
)

func fillAnnotations(ctx context.Context, rt *runtimepb.RuntimeConfig, env []*schema.BinaryConfig_EnvEntry) (map[string]string, error) {
	m := map[string]string{}

	for _, kv := range env {
		switch {
		case kv.ExperimentalFromSecret != "":
			return nil, fnerrors.BadInputError("secrets not supported in this scope")

		case kv.ExperimentalFromDownwardsFieldPath != "":
			return nil, fnerrors.BadInputError("experimentalFromDownwardsFieldPath not supported in this scope")

		case kv.FromSecretRef != nil:
			return nil, fnerrors.BadInputError("secrets not supported in this scope")

		case kv.FromServiceEndpoint != nil:
			endpoint, err := runtime.SelectServiceValue(rt, kv.FromServiceEndpoint, runtime.SelectServiceEndpoint)
			if err != nil {
				return nil, err
			}
			m[kv.Name] = endpoint

		case kv.FromServiceIngress != nil:
			url, err := runtime.SelectServiceValue(rt, kv.FromServiceIngress, runtime.SelectServiceIngress)
			if err != nil {
				return nil, err
			}
			m[kv.Name] = url

		case kv.FromResourceField != nil:
			return nil, fnerrors.BadInputError("resources not supported in this scope")

		case kv.FromFieldSelector != nil:
			instance, err := runtime.SelectInstance(rt, kv.FromFieldSelector.Instance)
			if err != nil {
				return nil, err
			}

			x, err := resources.SelectField("fromFieldSelector", instance, kv.FromFieldSelector.FieldSelector)
			if err != nil {
				return nil, err
			}

			vv, err := resources.CoerceAsString(x)
			if err != nil {
				return nil, err
			}

			m[kv.Name] = vv

		default:
			m[kv.Name] = kv.Value
		}
	}

	return m, nil
}
