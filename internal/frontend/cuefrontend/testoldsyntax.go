// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type cueTestOld struct {
	Name    string     `json:"name"`
	Binary  *cueBinary `json:"binary"`
	Driver  *cueBinary `json:"driver"`
	Fixture cueFixture `json:"fixture"`
}

type cueFixture struct {
	ServersUnderTest []string `json:"serversUnderTest"`
}

// Old syntax
func parsecueTestOld(ctx context.Context, loc pkggraph.Location, parent, v *fncue.CueV) (*schema.Test, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	test := cueTestOld{}
	if err := v.Val.Decode(&test); err != nil {
		return nil, err
	}

	testDef := &schema.Test{
		Name:             test.Name,
		ServersUnderTest: test.Fixture.ServersUnderTest,
	}

	var err error
	if test.Driver != nil {
		testDef.Driver, err = test.Driver.ToSchema(loc)
	} else if test.Binary != nil {
		testDef.Driver, err = test.Binary.ToSchema(loc)
	}
	if err != nil {
		return nil, err
	}

	return testDef, nil
}
