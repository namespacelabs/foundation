// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"strings"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
)

type cueSecret struct {
	Description string             `json:"description,omitempty"`
	Generate    *cueSecretGenerate `json:"generate,omitempty"`
}

type cueSecretGenerate struct {
	UniqueID        string `json:"uniqueId,omitempty"`
	RandomByteCount int    `json:"randomByteCount,omitempty"`
	Format          string `json:"format,omitempty"`
}

func parseSecrets(ctx context.Context, v *fncue.CueV) ([]*schema.SecretSpec, error) {
	var parsedSecrets []*schema.SecretSpec
	it, err := v.Val.Fields()
	if err != nil {
		return nil, err
	}

	for it.Next() {
		parsedSecret, err := parseSecret(ctx, it.Label(), it.Value())
		if err != nil {
			return nil, err
		}

		parsedSecrets = append(parsedSecrets, parsedSecret)
	}

	return parsedSecrets, nil
}

func parseSecret(ctx context.Context, name string, v cue.Value) (*schema.SecretSpec, error) {
	var bits cueSecret
	if err := v.Decode(&bits); err != nil {
		return nil, err
	}

	spec := &schema.SecretSpec{
		Name:        name,
		Description: bits.Description,
	}

	if bits.Generate != nil {
		spec.Generate = &schema.SecretSpec_GenerateSpec{
			UniqueId:        bits.Generate.UniqueID,
			RandomByteCount: int32(bits.Generate.RandomByteCount),
		}

		if spec.Generate.UniqueId == "" {
			return nil, fnerrors.BadInputError("%s: secrets with a generation specification require a uniqueId to be set", name)
		}

		if bits.Generate.Format != "" {
			x, ok := schema.SecretSpec_GenerateSpec_Format_value[strings.ToUpper(bits.Generate.Format)]
			if !ok {
				return nil, fnerrors.BadInputError("%s: no such format %q", name, bits.Generate.Format)
			}

			spec.Generate.Format = schema.SecretSpec_GenerateSpec_Format(x)
		} else {
			spec.Generate.Format = schema.SecretSpec_GenerateSpec_FORMAT_BASE64
		}
	}

	return spec, nil
}
