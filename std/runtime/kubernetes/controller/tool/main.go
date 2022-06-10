// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
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

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Admin Namespace",
		Resource:    applycorev1.Namespace(kubedef.AdminNamespace),
	})

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Admin Service Account",
		Resource:    applycorev1.ServiceAccount(serviceAccount, kubedef.AdminNamespace),
	})

	role := adminRole(serviceAccount)
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Admin Cluster Role",
		Resource: applyrbacv1.ClusterRole(role).WithRules(
			applyrbacv1.PolicyRule().WithAPIGroups("").WithResources("namespaces").WithVerbs("watch", "delete"),
			applyrbacv1.PolicyRule().WithAPIGroups("").WithResources("pods").WithVerbs("watch"),
			applyrbacv1.PolicyRule().WithAPIGroups("apps").WithResources("deployments").WithVerbs("watch", "list", "delete"),
		),
	})

	binding := adminBinding(serviceAccount)
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Admin Cluster Role Binding",
		Resource: applyrbacv1.ClusterRoleBinding(binding).
			WithRoleRef(applyrbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(role)).
			WithSubjects(applyrbacv1.Subject().
				WithKind("ServiceAccount").
				WithNamespace(kubedef.AdminNamespace).
				WithName(serviceAccount)),
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

// We use separate roles/accs to maintain a minimum set of permissions per usecase.
// This also removes the need to merge permissions on updates.
func adminRole(name string) string {
	return fmt.Sprintf("foundation:admin:%s-role", name)
}

func adminBinding(name string) string {
	return fmt.Sprintf("foundation:admin:%s-binding", name)
}
