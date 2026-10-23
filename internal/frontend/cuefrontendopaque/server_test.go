// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestPreferredNodeAffinity(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input string
		want  map[string]string
	}{
		{"omitted", `{}`, nil},
		{"empty", `{preferredNodeAffinity: {}}`, map[string]string{}},
		{"labels", `{preferredNodeAffinity: {"example.com/bandwidth": "200", "example.com/rack": "a"}}`, map[string]string{"example.com/bandwidth": "200", "example.com/rack": "a"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := cuecontext.New().CompileString(tt.input)
			if !assert.NoError(t, v.Err()) {
				return
			}
			fragment, err := parseCueServerExtension(context.Background(), nil, nil,
				&pkggraph.Package{Location: pkggraph.Location{PackageName: "example.com/server"}}, &fncue.CueV{Val: v})
			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, fragment.PreferredNodeAffinity)
			}
		})
	}
}
