// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

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
	serviceAccount := makeServiceAccount(r.Focus.Server)

	out.Definitions = append(out.Definitions, kubedef.Admin{
		Description:    "Ephemeral Controller",
		ServiceAccount: serviceAccount,
		Rules: []*applyrbacv1.PolicyRuleApplyConfiguration{
			applyrbacv1.PolicyRule().WithAPIGroups("").WithResources("namespaces").WithVerbs("list", "watch", "delete"),
			applyrbacv1.PolicyRule().WithAPIGroups("").WithResources("events").WithVerbs("watch"),
		},
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			ServiceAccount: serviceAccount,
		},
	})

	return nil
}

func (tool) Delete(ctx context.Context, r configure.StackRequest, out *configure.DeleteOutput) error {
	return nil
}

func makeServiceAccount(srv *schema.Server) string {
	return fmt.Sprintf("admin-%s", kubedef.MakeDeploymentId(srv))
}
