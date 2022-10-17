// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package source

import (
	"testing"

	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
)

func TestGeneratesValidServerId(t *testing.T) {
	s := schema.Server{
		Id: newId(),
	}

	if err := parsing.ValidateServerID(&s); err != nil {
		t.Errorf("%v", err)
	}
}
