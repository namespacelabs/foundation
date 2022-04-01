// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"

	"cuelang.org/go/cue"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

type cueTest struct {
	Name    string         `json:"name"`
	Binary  *schema.Binary `json:"binary"`
	Fixture cueFixture     `json:"fixture"`
}

type cueFixture struct {
	ServersUnderTest []string `json:"serversUnderTest"`
}

func parseCueTest(ctx context.Context, loc workspace.Location, parent, v *fncue.CueV) (*schema.Test, error) {
	// Ensure all fields are bound.
	if err := v.Val.Validate(cue.Concrete(true)); err != nil {
		return nil, err
	}

	test := cueTest{Binary: &schema.Binary{}}
	if err := v.Val.Decode(&test); err != nil {
		return nil, err
	}

	testDef := &schema.Test{
		Name:             test.Name,
		Binary:           test.Binary,
		ServersUnderTest: test.Fixture.ServersUnderTest,
	}

	if err := workspace.TransformTest(loc, testDef); err != nil {
		return nil, err
	}

	return testDef, nil
}
