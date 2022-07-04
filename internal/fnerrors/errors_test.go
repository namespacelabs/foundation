package fnerrors

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestErrorFormatting(t *testing.T) {
	cases := []struct {
		err      error
		expected string
	}{
		{err: UsageError("Run 'foobar'.", "It expired."),
			expected: "Failed: It expired.\n\n  Run 'foobar'.\n"},
		{err: UserError(nil, "wrapping it: %w", UsageError("Run 'foobar'.", "It expired.")),
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
