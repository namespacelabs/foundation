// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontendopaque

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueSecret struct {
	Description string `json:"description,omitempty"`
}

func parseSecrets(ctx context.Context, loc pkggraph.Location, v *fncue.CueV) ([]*schema.SecretSpec, error) {
	var parsedSecrets []*schema.SecretSpec
	it, err := v.Val.Fields()
	if err != nil {
		return nil, err
	}

	for it.Next() {
		parsedSecret, err := parseSecret(ctx, loc, it.Label(), it.Value())
		if err != nil {
			return nil, err
		}

		parsedSecrets = append(parsedSecrets, parsedSecret)
	}

	return parsedSecrets, nil
}

func parseSecret(ctx context.Context, loc pkggraph.Location, name string, v cue.Value) (*schema.SecretSpec, error) {
	var bits cueSecret
	if err := v.Decode(&bits); err != nil {
		return nil, err
	}

	return &schema.SecretSpec{
		Owner:       loc.PackageName.String(),
		Name:        name,
		Description: bits.Description,
	}, nil
}
