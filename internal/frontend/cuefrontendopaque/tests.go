// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/parsing/integration/api"
	"namespacelabs.dev/foundation/internal/protos"
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

	binaryRef, err := binary.ParseImage(ctx, env, pl, pkg, name, v, binary.ParseImageOpts{Required: false})
	if err != nil {
		return nil, fnerrors.NewWithLocation(pkg.Location, "parsing test %q failed: %w", name, err)
	}

	if binaryRef != nil {
		// TODO: use a PackageRef for the test driver binary instead of adding and then removing it from package binaries.
		if err := api.SetTestDriver(pkg.Location, out, pkg.Binaries[len(pkg.Binaries)-1]); err != nil {
			return nil, err
		}
		pkg.Binaries = pkg.Binaries[:len(pkg.Binaries)-1]
	}

	if i := v.LookupPath("integration"); i.Exists() {
		integration, err := integrationparsing.IntegrationParser.ParseEntity(ctx, env, pl, pkg.Location, i)
		if err != nil {
			return nil, err
		}

		out.Integration = &schema.Integration{
			Data: protos.WrapAnyOrDie(integration.Data),
		}
	}

	return out, nil
}
