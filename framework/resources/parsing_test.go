// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func TestSelector(t *testing.T) {
	parsed, err := ParseResourceData([]byte(`{
		"foobar": {
			"path": "path/to/foo",
			"child": {
				"endpoint": "xyz"
			},
			"another": {
				"isnull": null
			}
		},
		"wrong": "123"
	}`))
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		Resource  string
		Selector  string
		Expected  any
		ErrorCode codes.Code
	}{
		{"foobar", "path", "path/to/foo", codes.OK},
		{"foobar", "child.endpoint", "xyz", codes.OK},
		{"foobar", "child.example", nil, codes.NotFound},
		{"foobar", "another.isnull", nil, codes.NotFound},
		{"wrong", "foobar", nil, codes.InvalidArgument},
		{"doesntexist", "foobar", nil, codes.NotFound},
	} {
		got, err := parsed.SelectField(test.Resource, test.Selector)
		st, _ := status.FromError(err)
		if st.Code() != test.ErrorCode {
			t.Errorf("expected error code %v got %v", test.ErrorCode, st.Code())
			continue
		}

		if err == nil {
			if d := cmp.Diff(test.Expected, got); d != "" {
				t.Errorf("mismatch (-want +got):\n%s", d)
			}
		}
	}
}
