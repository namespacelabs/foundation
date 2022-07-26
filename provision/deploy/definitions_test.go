// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/testing/protocmp"
	"namespacelabs.dev/foundation/internal/frontend"
	"namespacelabs.dev/foundation/internal/stack"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
)

func TestEnsureInvocationOrder(t *testing.T) {
	stack := &stack.Stack{
		Servers: []provision.Server{{
			Location: workspace.Location{
				PackageName: "a",
			},
		}, {
			Location: workspace.Location{
				PackageName: "b",
			},
		}, {
			Location: workspace.Location{
				PackageName: "c",
			},
		}},
		ParsedServers: []*stack.ParsedServer{{
			Deps: []*stack.ParsedNode{{
				ProvisionPlan: frontend.ProvisionPlan{
					PreparedProvisionPlan: frontend.PreparedProvisionPlan{
						ProvisionStack: frontend.ProvisionStack{
							DeclaredStack: []schema.PackageName{"b", "c"},
						},
					},
				},
			}},
		}, {
			Deps: []*stack.ParsedNode{{
				ProvisionPlan: frontend.ProvisionPlan{
					PreparedProvisionPlan: frontend.PreparedProvisionPlan{
						ProvisionStack: frontend.ProvisionStack{
							DeclaredStack: []schema.PackageName{"c"},
						},
					},
				},
			}},
		}, {
			Deps: []*stack.ParsedNode{{
				ProvisionPlan: frontend.ProvisionPlan{
					PreparedProvisionPlan: frontend.PreparedProvisionPlan{
						ProvisionStack: frontend.ProvisionStack{
							DeclaredStack: []schema.PackageName{},
						},
					},
				},
			}},
		}},
	}

	perServer := map[schema.PackageName]*serverDefs{
		"a": {Ops: []*schema.SerializedInvocation{{Description: "a"}}},
		"b": {Ops: []*schema.SerializedInvocation{{Description: "b"}}},
		"c": {Ops: []*schema.SerializedInvocation{{Description: "c"}}},
	}

	got, err := ensureInvocationOrder(context.Background(), stack, perServer)
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
