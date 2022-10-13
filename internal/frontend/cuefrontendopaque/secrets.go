// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
)

type cueSecret struct {
	Description string `json:"description,omitempty"`
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

	return &schema.SecretSpec{
		Name:        name,
		Description: bits.Description,
	}, nil
}
