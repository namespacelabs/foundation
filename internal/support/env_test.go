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
		[]*schema.BinaryConfig_EnvEntry{{Name: "COLLIDE", Value: str("foo")}, {Name: "COLLIDE", Value: str("bar")}},
		[]*schema.BinaryConfig_EnvEntry{{Name: "OK1", Value: str("foo")}, {Name: "OK2", Value: str("baz")}},
	); err == nil {
		t.Fatalf("expected merge to fail")
	}

	if _, err := MergeEnvs(
		[]*schema.BinaryConfig_EnvEntry{{Name: "COLLIDE", Value: str("foo")}, {Name: "OK1", Value: str("bar")}},
		[]*schema.BinaryConfig_EnvEntry{{Name: "OK2", Value: str("foo")}, {Name: "COLLIDE", Value: str("baz")}},
	); err == nil {
		t.Fatalf("expected merge to fail")
	}

	if _, err := MergeEnvs(
		[]*schema.BinaryConfig_EnvEntry{{Name: "OK1", Value: str("foo")}, {Name: "OK2", Value: str("bar")}},
		[]*schema.BinaryConfig_EnvEntry{{Name: "OK3", Value: str("foo")}, {Name: "OK4", Value: str("baz")}},
	); err != nil {
		t.Fatal(err)
	}
}

func str(str string) *schema.Resolvable {
	return &schema.Resolvable{Value: str}
}
