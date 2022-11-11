// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package support

import (
	"testing"

	"namespacelabs.dev/foundation/schema"
)

func TestMerge(t *testing.T) {
	if _, err := MergeEnvs(
		[]*schema.BinaryConfig_EnvEntry{{Name: "COLLIDE", Value: "foo"}, {Name: "COLLIDE", Value: "bar"}},
		[]*schema.BinaryConfig_EnvEntry{{Name: "OK1", Value: "foo"}, {Name: "OK2", Value: "baz"}},
	); err == nil {
		t.Fatalf("expected merge to fail")
	}

	if _, err := MergeEnvs(
		[]*schema.BinaryConfig_EnvEntry{{Name: "COLLIDE", Value: "foo"}, {Name: "OK1", Value: "bar"}},
		[]*schema.BinaryConfig_EnvEntry{{Name: "OK2", Value: "foo"}, {Name: "COLLIDE", Value: "baz"}},
	); err == nil {
		t.Fatalf("expected merge to fail")
	}

	if _, err := MergeEnvs(
		[]*schema.BinaryConfig_EnvEntry{{Name: "OK1", Value: "foo"}, {Name: "OK2", Value: "bar"}},
		[]*schema.BinaryConfig_EnvEntry{{Name: "OK3", Value: "foo"}, {Name: "OK4", Value: "baz"}},
	); err != nil {
		t.Fatal(err)
	}
}
