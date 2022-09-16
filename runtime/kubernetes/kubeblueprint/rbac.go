// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeblueprint

import (
	"fmt"

	"google.golang.org/grpc/codes"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/provision/configure"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/std/go/rpcerrors"
)

type Scope string

const (
	NamespaceScope Scope = "k8s.scope.namespace"
	ClusterScope   Scope = "k8s.scope.cluster"
)

type GrantKubeACLs struct {
	DescriptionBase string
	ServiceAccount  string
	Rules           []*rbacv1.PolicyRuleApplyConfiguration
}

func (g GrantKubeACLs) Compile(req configure.StackRequest, scope Scope, out *configure.ApplyOutput) error {
	if g.Rules == nil {
		return fnerrors.BadInputError("Rules is required")
	}

	if scope != NamespaceScope && scope != ClusterScope {
		return fnerrors.BadInputError("%s: unsupported scope", scope)
	}

	roleName, roleBinding := makeRoles(req)
	labels := kubedef.MakeLabels(req.Env, req.Focus.Server)

	kr, err := kubetool.FromRequest(req)
	if err != nil {
		return err
	}

	serviceAccount := g.serviceAccount(req)

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description:  fmt.Sprintf("%s: Service Account", g.DescriptionBase),
		SetNamespace: kr.CanSetNamespace,
		Resource: corev1.ServiceAccount(serviceAccount, kr.Namespace).
			WithLabels(kubedef.MakeLabels(req.Env, req.Focus.Server)).
			WithAnnotations(kubedef.BaseAnnotations()),
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			ServiceAccount: serviceAccount,
		},
	})

	if kr.Context.GetHasApplyRoleBinding() {
		out.Invocations = append(out.Invocations, kubedef.ApplyRoleBinding{
			Description:     fmt.Sprintf("%s: Role", g.DescriptionBase),
			Namespaced:      scope == NamespaceScope,
			RoleName:        roleName,
			RoleBindingName: roleBinding,
			Rules:           g.Rules,
			Labels:          labels,
			Annotations:     kubedef.BaseAnnotations(),
			ServiceAccount:  serviceAccount,
		})

		return nil
	}

	if kr.Namespace == "" {
		return rpcerrors.Errorf(codes.FailedPrecondition, "kubernetes namespace missing")
	}

	switch scope {
	case NamespaceScope:
		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description:  fmt.Sprintf("%s: Role", g.DescriptionBase),
			SetNamespace: kr.CanSetNamespace,
			Resource: rbacv1.Role(roleName, kr.Namespace).
				WithRules(g.Rules...).
				WithLabels(labels).
				WithAnnotations(kubedef.BaseAnnotations()),
		})

		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description:  fmt.Sprintf("%s: Role Binding", g.DescriptionBase),
			SetNamespace: kr.CanSetNamespace,
			Resource: rbacv1.RoleBinding(roleBinding, kr.Namespace).
				WithLabels(labels).
				WithAnnotations(kubedef.BaseAnnotations()).
				WithRoleRef(rbacv1.RoleRef().
					WithAPIGroup("rbac.authorization.k8s.io").
					WithKind("Role").
					WithName(roleName)).
				WithSubjects(rbacv1.Subject().
					WithKind("ServiceAccount").
					WithNamespace(kr.Namespace).
					WithName(serviceAccount)),
		})

	case ClusterScope:
		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Cluster Role", g.DescriptionBase),
			Resource: rbacv1.ClusterRole(roleName).
				WithRules(g.Rules...).
				WithLabels(labels).
				WithAnnotations(kubedef.BaseAnnotations()),
		})

		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Cluster Role Binding", g.DescriptionBase),
			Resource: rbacv1.ClusterRoleBinding(roleBinding).
				WithLabels(labels).
				WithAnnotations(kubedef.BaseAnnotations()).
				WithRoleRef(rbacv1.RoleRef().
					WithAPIGroup("rbac.authorization.k8s.io").
					WithKind("ClusterRole").
					WithName(roleName)).
				WithSubjects(rbacv1.Subject().
					WithKind("ServiceAccount").
					WithNamespace(kr.Namespace).
					WithName(serviceAccount)),
		})
	}

	return nil
}

func makeRoles(req configure.StackRequest) (string, string) {
	roleName := fmt.Sprintf("foundation:managed:%s", kubedef.MakeDeploymentId(req.Focus.Server))
	roleBinding := fmt.Sprintf("foundation:managed:%s", kubedef.MakeDeploymentId(req.Focus.Server))

	return roleName, roleBinding
}

func (g GrantKubeACLs) serviceAccount(req configure.StackRequest) string {
	serviceAccount := g.ServiceAccount
	if serviceAccount == "" {
		serviceAccount = kubedef.MakeDeploymentId(req.Focus.Server)
	}
	return serviceAccount
}
