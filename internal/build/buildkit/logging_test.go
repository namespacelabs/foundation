// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestTimestampLogger(t *testing.T) {
	var x int64
	now := func() time.Time {
		t := time.Unix(1656887243+x, x*10+1).UTC()
		x++
		return t
	}

	var out bytes.Buffer
	w := &timestampPrefixWriter{&out, now, true}

	fmt.Fprint(w, "foo\n")
	fmt.Fprint(w, "bar\nxyz\n")
	fmt.Fprint(w, "a")
	fmt.Fprint(w, "b")
	fmt.Fprint(w, "c\n")

	if d := cmp.Diff(`2022-07-03T22:27:23.000000001Z foo
2022-07-03T22:27:24.000000011Z bar
2022-07-03T22:27:25.000000021Z xyz
2022-07-03T22:27:26.000000031Z abc
`, out.String()); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
