// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package format

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"namespacelabs.dev/foundation/internal/fnerrors"
)

func TestErrorFormatting(t *testing.T) {
	cases := []struct {
		err      error
		expected string
	}{
		{err: fnerrors.UsageError("Run 'foobar'.", "It expired."),
			expected: "Failed: It expired.\n\n  Run 'foobar'.\n"},
		{err: fnerrors.UserError(nil, "wrapping it: %w", fnerrors.UsageError("Run 'foobar'.", "It expired.")),
			expected: "Failed: wrapping it: It expired.\n\n  Run 'foobar'.\n"},
	}

	for _, c := range cases {
		var out bytes.Buffer
		Format(&out, c.err)

		if d := cmp.Diff(c.expected, out.String()); d != "" {
			t.Errorf("mismatch (-want +got):\n%s", d)
		}
	}
}
