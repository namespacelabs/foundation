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
	"namespacelabs.dev/foundation/provision/parsed"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestEnsureInvocationOrder(t *testing.T) {
	stack := &provision.Stack{
		Servers: []provision.Server{{
			Server: parsed.Server{
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
			Server: parsed.Server{
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
			Server: parsed.Server{
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
