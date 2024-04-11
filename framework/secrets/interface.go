// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type SecretsSource interface {
	Load(context.Context, pkggraph.ModuleResolver, *SecretLoadRequest) (*schema.SecretResult, error)
	MissingError(*schema.PackageRef, *schema.SecretSpec, schema.PackageName) error
}

type SecretLoadRequest struct {
	SecretRef *schema.PackageRef
	Server    *ServerRef
}

type ServerRef struct {
	PackageName schema.PackageName
	ModuleName  string
	RelPath     string // Relative path within the module.
}

type GroundedSecrets interface {
	Get(ctx context.Context, secretRef *schema.PackageRef) (*schema.SecretResult, error)
}

func Load(ctx context.Context, src SecretsSource, mods pkggraph.ModuleResolver, owner schema.PackageNameLike, ref string) (*schema.SecretResult, error) {
	resolved, err := schema.ParsePackageRef(owner, ref)
	if err != nil {
		return nil, err
	}

	return src.Load(ctx, mods, &SecretLoadRequest{
		SecretRef: &schema.PackageRef{
			PackageName: resolved.PackageName,
			Name:        resolved.Name,
		},
	})
}
