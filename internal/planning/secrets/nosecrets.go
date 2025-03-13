// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"

	"namespacelabs.dev/foundation/framework/secrets"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

var NoSecrets noSecrets

type noSecrets struct{}

var _ secrets.SecretsSource = noSecrets{}

func (noSecrets) Load(context.Context, pkggraph.ModuleResolver, *secrets.SecretLoadRequest) (*schema.SecretResult, error) {
	return nil, fnerrors.Newf("secrets are not available in this path")
}

func (noSecrets) MissingError(*schema.PackageRef, *schema.SecretSpec, schema.PackageName) error {
	return fnerrors.InternalError("secrets are not available")
}
