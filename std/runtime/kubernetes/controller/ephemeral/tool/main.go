// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"

	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

type tool struct{}

func main() {
	h := configure.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	configure.Handle(h)
}

func (tool) Apply(ctx context.Context, r configure.StackRequest, out *configure.ApplyOutput) error {
	out.Definitions = append(out.Definitions, kubedef.Admin{
		Description: "Ephemeral Controller",
		Name:        "ephemeralcontroller",
		Rules: []*applyrbacv1.PolicyRuleApplyConfiguration{
			applyrbacv1.PolicyRule().WithAPIGroups("").WithResources("namespaces").WithVerbs("list", "delete"),
			applyrbacv1.PolicyRule().WithAPIGroups("").WithResources("events").WithVerbs("list"),
		},
	})

	return nil
}

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return nil
}
