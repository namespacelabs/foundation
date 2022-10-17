// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestEnsureInvocationOrder(t *testing.T) {
	stack := &provision.Stack{
		Servers: []provision.PlannedServer{{
			Server: provision.Server{
				Location: pkggraph.Location{
					PackageName: "a",
				},
			},
			ParsedDeps: []*provision.ParsedNode{{
				ProvisionPlan: pkggraph.ProvisionPlan{
					PreparedProvisionPlan: pkggraph.PreparedProvisionPlan{
						DeclaredStack: []schema.PackageName{"b", "c"},
					},
				},
			}},
		}, {
			Server: provision.Server{
				Location: pkggraph.Location{
					PackageName: "b",
				},
			},
			ParsedDeps: []*provision.ParsedNode{{
				ProvisionPlan: pkggraph.ProvisionPlan{
					PreparedProvisionPlan: pkggraph.PreparedProvisionPlan{
						DeclaredStack: []schema.PackageName{"c"},
					},
				},
			}},
		}, {
			Server: provision.Server{
				Location: pkggraph.Location{
					PackageName: "c",
				},
			},
			ParsedDeps: []*provision.ParsedNode{{
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
