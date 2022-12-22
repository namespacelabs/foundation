// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// Common parsing code between new and old syntaxes.
func ParsePackage(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, v *fncue.CueV, loc pkggraph.Location) (*pkggraph.Package, error) {
	parsedPkg := &pkggraph.Package{
		Location: loc,
	}

	if tests := v.LookupPath("tests"); tests.Exists() {
		parsedTests, err := parseTests(ctx, env, pl, parsedPkg, tests)
		if err != nil {
			return nil, err
		}

		parsedPkg.Tests = append(parsedPkg.Tests, parsedTests...)
	}

	if r := v.LookupPath("resourceClasses"); r.Exists() {
		it, err := r.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}
			parsedRc, err := parseResourceClass(ctx, loc, it.Label(), val)
			if err != nil {
				return nil, err
			}

			parsedPkg.ResourceClassSpecs = append(parsedPkg.ResourceClassSpecs, parsedRc)
		}
	}

	if r := v.LookupPath("providers"); r.Exists() {
		it, err := r.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}

			parsedProvider, err := parseResourceProvider(ctx, env, pl, parsedPkg, it.Label(), val)
			if err != nil {
				return nil, err
			}

			parsedPkg.ResourceProvidersSpecs = append(parsedPkg.ResourceProvidersSpecs, parsedProvider)
		}
	}

	if r := v.LookupPath("resources"); r.Exists() {
		it, err := r.Val.Fields()
		if err != nil {
			return nil, err
		}

		for it.Next() {
			val := &fncue.CueV{Val: it.Value()}
			parsedResource, err := ParseResourceInstanceFromCue(ctx, env, pl, parsedPkg, it.Label(), val)
			if err != nil {
				return nil, err
			}

			parsedPkg.ResourceInstanceSpecs = append(parsedPkg.ResourceInstanceSpecs, parsedResource)
		}
	}

	if volumes := v.LookupPath("volumes"); volumes.Exists() {
		parsedVolumes, err := ParseVolumes(ctx, pl, loc, volumes)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing volumes failed: %w", err)
		}
		parsedPkg.Volumes = append(parsedPkg.Volumes, parsedVolumes...)
	}

	if secrets := v.LookupPath("secrets"); secrets.Exists() {
		secretSpecs, err := parseSecrets(ctx, loc, secrets)
		if err != nil {
			return nil, err
		}

		parsedPkg.Secrets = append(parsedPkg.Secrets, secretSpecs...)
	}

	// Binaries should really be called "OCI Images".
	if binary := v.LookupPath("binary"); binary.Exists() {
		parsedBinary, err := parseCueBinary(ctx, loc, v, binary)
		if err != nil {
			return nil, fnerrors.NewWithLocation(loc, "parsing binary: %w", err)
		}
		parsedPkg.Binaries = append(parsedPkg.Binaries, parsedBinary)
	}

	return parsedPkg, nil
}
