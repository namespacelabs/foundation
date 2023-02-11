// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package args

import (
	"context"

	"namespacelabs.dev/foundation/internal/parsing/invariants"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type ResolvableValue struct {
	value                              string
	fromSecret                         string
	fromServiceEndpoint                string
	fromServiceIngress                 string
	experimentalFromDownwardsFieldPath string
	fromField                          *fromFieldSyntax
	fromResourceField                  *resourceFieldSyntax
}

func (value *ResolvableValue) ToProto(ctx context.Context, pl pkggraph.PackageLoader, owner schema.PackageName) (*schema.Resolvable, error) {
	out := &schema.Resolvable{}

	switch {
	case value.value != "":
		out.Value = value.value

	case value.fromSecret != "":
		secretRef, err := schema.ParsePackageRef(owner, value.fromSecret)
		if err != nil {
			return nil, err
		}
		if err := invariants.EnsurePackageLoaded(ctx, pl, owner, secretRef); err != nil {
			return nil, err
		}

		out.FromSecretRef = secretRef

	case value.fromServiceEndpoint != "":
		serviceRef, err := schema.ParsePackageRef(owner, value.fromServiceEndpoint)
		if err != nil {
			return nil, err
		}
		if err := invariants.EnsurePackageLoaded(ctx, pl, owner, serviceRef); err != nil {
			return nil, err
		}

		out.FromServiceEndpoint = &schema.ServiceRef{
			ServerRef:   &schema.PackageRef{PackageName: serviceRef.PackageName},
			ServiceName: serviceRef.Name,
		}

	case value.fromServiceIngress != "":
		serviceRef, err := schema.ParsePackageRef(owner, value.fromServiceIngress)
		if err != nil {
			return nil, err
		}
		if err := invariants.EnsurePackageLoaded(ctx, pl, owner, serviceRef); err != nil {
			return nil, err
		}
		out.FromServiceIngress = &schema.ServiceRef{
			ServerRef:   &schema.PackageRef{PackageName: serviceRef.PackageName},
			ServiceName: serviceRef.Name,
		}

	case value.fromResourceField != nil:
		resourceRef, err := schema.ParsePackageRef(owner, value.fromResourceField.Resource)
		if err != nil {
			return nil, err
		}
		if err := invariants.EnsurePackageLoaded(ctx, pl, owner, resourceRef); err != nil {
			return nil, err
		}

		out.FromResourceField = &schema.ResourceConfigFieldSelector{
			Resource:      resourceRef,
			FieldSelector: value.fromResourceField.FieldRef,
		}

	case value.fromField != nil:
		x, err := value.fromField.ToProto(ctx, pl, owner)
		if err != nil {
			return nil, err
		}
		out.FromFieldSelector = x

	case value.experimentalFromDownwardsFieldPath != "":
		out.ExperimentalFromDownwardsFieldPath = value.experimentalFromDownwardsFieldPath
	}

	return out, nil
}
