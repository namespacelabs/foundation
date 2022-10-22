// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestEnsureInvocationOrder(t *testing.T) {
	stack := &planning.Stack{
		Servers: []planning.PlannedServer{{
			Server: planning.Server{
				Location: pkggraph.Location{
					PackageName: "a",
				},
			},
			ParsedDeps: []*planning.ParsedNode{{
				ProvisionPlan: pkggraph.ProvisionPlan{
					PreparedProvisionPlan: pkggraph.PreparedProvisionPlan{
						DeclaredStack: []schema.PackageName{"b", "c"},
					},
				},
			}},
		}, {
			Server: planning.Server{
				Location: pkggraph.Location{
					PackageName: "b",
				},
			},
			ParsedDeps: []*planning.ParsedNode{{
				ProvisionPlan: pkggraph.ProvisionPlan{
					PreparedProvisionPlan: pkggraph.PreparedProvisionPlan{
						DeclaredStack: []schema.PackageName{"c"},
					},
				},
			}},
		}, {
			Server: planning.Server{
				Location: pkggraph.Location{
					PackageName: "c",
				},
			},
			ParsedDeps: []*planning.ParsedNode{{
				ProvisionPlan: pkggraph.ProvisionPlan{
					PreparedProvisionPlan: pkggraph.PreparedProvisionPlan{
						DeclaredStack: []schema.PackageName{},
					},
				},
			}},
		}},
	}

	perServerOps := map[schema.PackageName][]*schema.SerializedInvocation{
		"a": {{Description: "a"}},
		"b": {{Description: "b"}},
		"c": {{Description: "c"}},
	}

	got, err := ensureInvocationOrder(context.Background(), stack, perServerOps)
	if err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff([]*schema.SerializedInvocation{
		{Description: "c"},
		{Description: "b"},
		{Description: "a"},
	}, got, protocmp.Transform()); d != "" {
		t.Errorf("mismatch (-want +got):\n%s", d)
	}
}
