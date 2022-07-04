// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeblueprint

import (
	"fmt"

	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
)

type Scope string

const (
	NamespaceScope Scope = "k8s.scope.namespace"
	ClusterScope   Scope = "k8s.scope.cluster"
)

type GrantKubeACLs struct {
	DescriptionBase string
	ServiceAccount  string
	Scope           Scope
	Rules           []*rbacv1.PolicyRuleApplyConfiguration
}

func (g GrantKubeACLs) Compile(req configure.StackRequest, out *configure.ApplyOutput) error {
	serviceAccount := g.ServiceAccount
	if serviceAccount == "" {
		serviceAccount = kubedef.MakeDeploymentId(req.Focus.Server)
	}

	roleName := fmt.Sprintf("foundation:managed:%s", kubedef.MakeDeploymentId(req.Focus.Server))
	roleBinding := fmt.Sprintf("foundation:managed:%s", kubedef.MakeDeploymentId(req.Focus.Server))

	if g.Rules == nil {
		return fnerrors.BadInputError("Rules is required")
	}

	if g.Scope != NamespaceScope && g.Scope != ClusterScope {
		return fnerrors.BadInputError("Scope must be Namespace or Cluster")
	}

	namespace := kubetool.FromRequest(req).Namespace
	labels := kubedef.MakeLabels(req.Env, req.Focus.Server)

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: fmt.Sprintf("%s: Service Account", g.DescriptionBase),
		Resource:    corev1.ServiceAccount(serviceAccount, namespace).WithLabels(labels),
	})

	switch g.Scope {
	case NamespaceScope:
		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Role", g.DescriptionBase),
			Resource:    rbacv1.Role(roleName, namespace).WithRules(g.Rules...).WithLabels(labels),
		})

		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s:  Role Binding", g.DescriptionBase),
			Resource: rbacv1.RoleBinding(roleBinding, namespace).
				WithLabels(labels).
				WithRoleRef(rbacv1.RoleRef().
					WithAPIGroup("rbac.authorization.k8s.io").
					WithKind("Role").
					WithName(roleName)).
				WithSubjects(rbacv1.Subject().
					WithKind("ServiceAccount").
					WithNamespace(namespace).
					WithName(serviceAccount)),
		})

	case ClusterScope:
		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Cluster Role", g.DescriptionBase),
			Resource:    rbacv1.ClusterRole(roleName).WithRules(g.Rules...).WithLabels(labels),
		})

		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Cluster Role Binding", g.DescriptionBase),
			Resource: rbacv1.ClusterRoleBinding(roleBinding).
				WithLabels(labels).
				WithRoleRef(rbacv1.RoleRef().
					WithAPIGroup("rbac.authorization.k8s.io").
					WithKind("ClusterRole").
					WithName(roleName)).
				WithSubjects(rbacv1.Subject().
					WithKind("ServiceAccount").
					WithNamespace(namespace).
					WithName(serviceAccount)),
		})
	}

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			ServiceAccount: serviceAccount,
		},
	})

	return nil
}
