// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package main

import (
	"context"
	"fmt"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
)

type tool struct{}

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(tool{})
	provisioning.Handle(h)
}

func (tool) Apply(ctx context.Context, r provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	serviceAccount := makeServiceAccount(r.Focus.Server)

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Admin Namespace",
		Resource: applycorev1.Namespace(kubedef.AdminNamespace).
			WithAnnotations(kubedef.BaseAnnotations()),
	})

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Admin Service Account",
		Resource: applycorev1.ServiceAccount(serviceAccount, kubedef.AdminNamespace).
			WithAnnotations(kubedef.BaseAnnotations()),
	})

	role := adminRole(serviceAccount)
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Admin Cluster Role",
		Resource: applyrbacv1.ClusterRole(role).
			WithAnnotations(kubedef.BaseAnnotations()).
			WithRules(
				// CRDs have their own API groups.
				applyrbacv1.PolicyRule().WithAPIGroups("*").WithResources("*").
					WithVerbs("apply", "create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"),
				applyrbacv1.PolicyRule().WithNonResourceURLs("*").
					WithVerbs("apply", "create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"),
				// TODO permissions should be declarative (each node should tell which setup permissions it needs)
			),
	})

	binding := adminBinding(serviceAccount)
	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: "Admin Cluster Role Binding",
		Resource: applyrbacv1.ClusterRoleBinding(binding).
			WithAnnotations(kubedef.BaseAnnotations()).
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

func (tool) Delete(ctx context.Context, r provisioning.StackRequest, out *provisioning.DeleteOutput) error {
	return nil
}

func makeServiceAccount(srv runtime.Deployable) string {
	return fmt.Sprintf("admin-%s", kubedef.MakeDeploymentId(srv))
}

// We use separate roles/accs to maintain a minimum set of permissions per usecase.
// This also removes the need to merge permissions on updates.
func adminRole(name string) string {
	return fmt.Sprintf("ns:%s-role", name)
}

func adminBinding(name string) string {
	return fmt.Sprintf("ns:%s-binding", name)
}
