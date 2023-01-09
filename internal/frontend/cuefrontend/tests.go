// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/args"
	"namespacelabs.dev/foundation/internal/frontend/cuefrontend/binary"
	integrationparsing "namespacelabs.dev/foundation/internal/frontend/cuefrontend/integration/api"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

// Needs to be consistent with JSON names of cueSecret fields.
var testFields = []string{"serversUnderTest", "args", "env", "image", "imageFrom", "integration"}

type cueTest struct {
	Servers []string            `json:"serversUnderTest"`
	Args    *args.ArgsListOrMap `json:"args"`
	Env     *args.EnvMap        `json:"env"`
}

// New syntax
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
	if err := ValidateNoExtraFields(pkg.Location, fmt.Sprintf("test %q:", name) /* messagePrefix */, v, testFields); err != nil {
		return nil, err
	}

	var bits cueTest
	if err := v.Val.Decode(&bits); err != nil {
		return nil, err
	}

	envVars, err := bits.Env.Parsed(pkg.Location.PackageName)
	if err != nil {
		return nil, err
	}

	out := &schema.Test{
		Name:             name,
		ServersUnderTest: bits.Servers,
		BinaryConfig: &schema.BinaryConfig{
			Args: bits.Args.Parsed(),
			Env:  envVars,
		},
	}

	binaryRef, err := binary.ParseImage(ctx, env, pl, pkg, name, v, binary.ParseImageOpts{Required: false})
	if err != nil {
		return nil, fnerrors.NewWithLocation(pkg.Location, "parsing test %q failed: %w", name, err)
	}

	out.Driver = binaryRef

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
