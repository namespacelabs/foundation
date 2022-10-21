// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package resources

import (
	"testing"

	"namespacelabs.dev/foundation/library/runtime"
)

func TestParseResources(t *testing.T) {
	parsed, err := ParseResourceData([]byte(`{
		"foobar": {
			"path": "path/to/foo"
		}
	}`))
	if err != nil {
		t.Fatal(err)
	}

	secret := &runtime.SecretInstance{}
	if err := parsed.Unmarshal("foobar", secret); err != nil {
		t.Fatal(err)
	}

	const expected = "path/to/foo"
	if secret.Path != expected {
		t.Errorf("expected parsed path to be %q got %q", expected, secret.Path)
	}
}
