// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueTest struct {
	Servers []string `json:"serversUnderTest"`
}

func parseTests(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, v *fncue.CueV) ([]*schema.Test, error) {
	it, err := v.Val.Fields()
	if err != nil {
		return nil, err
	}

	out := []*schema.Test{}

	for it.Next() {
		parsedTest, err := parseTest(ctx, env, pl, pkg, it.Label(), (&fncue.CueV{Val: it.Value()}))
		if err != nil {
			return nil, err
		}

		out = append(out, parsedTest)
	}

	return out, nil
}

func parseTest(ctx context.Context, env *schema.Environment, pl parsing.EarlyPackageLoader, pkg *pkggraph.Package, name string, v *fncue.CueV) (*schema.Test, error) {
	var bits cueTest
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	out := &schema.Test{
		Name:             name,
		ServersUnderTest: bits.Servers,
	}

	binRef, err := ParseImage(ctx, env, pl, pkg, name, v)
	if err != nil {
		return nil, err
	}

	if binRef == nil {
		return nil, fnerrors.UserError(pkg.Location, "test %q: missing '%s' or 'image' definition", name, imageFromPath)
	}

	// TODO: use a PackageRef for the test driver binary instead of adding and then removing it from package binaries.
	out.Driver = pkg.Binaries[len(pkg.Binaries)-1]
	pkg.Binaries = pkg.Binaries[:len(pkg.Binaries)-1]

	return out, nil
}
