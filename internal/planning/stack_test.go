// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

func TestPreferredNodeAffinityFragments(t *testing.T) {
	for _, tt := range []struct {
		name                string
		first, second, want map[string]string
		conflict            bool
	}{
		{"extension only", nil, map[string]string{"bandwidth": "200"}, map[string]string{"bandwidth": "200"}, false},
		{"disjoint", map[string]string{"bandwidth": "200"}, map[string]string{"rack": "a"}, map[string]string{"bandwidth": "200", "rack": "a"}, false},
		{"identical", map[string]string{"bandwidth": "200"}, map[string]string{"bandwidth": "200"}, map[string]string{"bandwidth": "200"}, false},
		{"conflicting", map[string]string{"bandwidth": "200"}, map[string]string{"bandwidth": "100"}, nil, true},
		{"conflicting reverse", map[string]string{"bandwidth": "100"}, map[string]string{"bandwidth": "200"}, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := Server{
				Location: pkggraph.Location{PackageName: "example.com/server"},
				entry:    &schema.Stack_Entry{Server: &schema.Server{PackageName: "example.com/server"}},
				fragments: []*schema.ServerFragment{
					{MainContainer: &schema.Container{}, PreferredNodeAffinity: tt.first},
					{MainContainer: &schema.Container{}, PreferredNodeAffinity: tt.second},
				},
			}
			var state computeState
			var planned PlannedServer
			ctx := tasks.WithSink(context.Background(), tasks.NullSink())
			err := state.computeServerContents(ctx, nil, server, ProvisionOpts{}, &planned)
			if tt.conflict {
				assert.ErrorContains(t, err, `incompatible preferredNodeAffinity values for "bandwidth"`)
			} else if assert.NoError(t, err) {
				assert.Equal(t, tt.want, planned.MergedFragment.PreferredNodeAffinity)
			}
		})
	}
}
