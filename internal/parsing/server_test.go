// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package parsing

import (
	"testing"

	"namespacelabs.dev/foundation/schema"
)

func TestShortId(t *testing.T) {
	s := schema.Server{
		Id: "tooshort",
	}

	if err := ValidateServerID(&s); err == nil {
		t.Fail()
	}
}

func TestInvalidChar(t *testing.T) {
	s := schema.Server{
		Id: "correctlengthbut$isforbidden",
	}

	if err := ValidateServerID(&s); err == nil {
		t.Fail()
	}
}

func TestCapitalLetters(t *testing.T) {
	s := schema.Server{
		Id: "correctlengthbutCapital",
	}

	if err := ValidateServerID(&s); err == nil {
		t.Fail()
	}
}

func TestValidServerId(t *testing.T) {
	s := schema.Server{
		Id: "93some82valid14id42",
	}

	if err := ValidateServerID(&s); err != nil {
		t.Errorf("%v", err)
	}
}
