// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package oci

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFoo(t *testing.T) {
	for _, test := range []struct {
		ref      string
		expected ImageID
	}{
		{
			ref: "ghcr.io/namespacelabs/foundation/std/monitoring/prometheus/tool@sha256:dbf8827bc17a9f77bf500015331f02ad3b1570a43aa1d5a46196b0a61a2942b6",
			expected: ImageID{
				Repository: "ghcr.io/namespacelabs/foundation/std/monitoring/prometheus/tool",
				Digest:     "sha256:dbf8827bc17a9f77bf500015331f02ad3b1570a43aa1d5a46196b0a61a2942b6",
			},
		},
		{
			ref: "ghcr.io/namespacelabs/foundation/std/monitoring/prometheus/tool:v1",
			expected: ImageID{
				Repository: "ghcr.io/namespacelabs/foundation/std/monitoring/prometheus/tool",
				Tag:        "v1",
			},
		},
	} {
		got, err := ParseImageID(test.ref)
		if err != nil {
			t.Error(err)
			continue
		}

		if d := cmp.Diff(test.expected, got); d != "" {
			t.Errorf("mismatch (-want +got):\n%s", d)
		}
	}
}