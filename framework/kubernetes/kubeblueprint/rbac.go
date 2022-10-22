// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeblueprint

import (
	"fmt"

	"google.golang.org/grpc/codes"
	corev1 "k8s.io/client-go/applyconfigurations/core/v1"
	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubetool"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/framework/rpcerrors"
	"namespacelabs.dev/foundation/internal/fnerrors"
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

func (g GrantKubeACLs) Compile(req provisioning.StackRequest, scope Scope, out *provisioning.ApplyOutput) error {
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
			DescriptionBase: g.DescriptionBase,
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

	for _, apply := range MakeInvocations(g.DescriptionBase, scope, kr,
		roleName, roleBinding, labels, kubedef.BaseAnnotations(), serviceAccount, g.Rules) {
		out.Invocations = append(out.Invocations, apply)
	}

	return nil
}

func MakeInvocations(descriptionBase string, scope Scope, kr *kubetool.ContextualEnv, roleName, roleBinding string, labels, annotations map[string]string, serviceAccount string, rules []*rbacv1.PolicyRuleApplyConfiguration) []kubedef.Apply {
	var invocations []kubedef.Apply

	switch scope {
	case NamespaceScope:
		invocations = append(invocations, kubedef.Apply{
			Description:  fmt.Sprintf("%s: Role", descriptionBase),
			SetNamespace: kr.CanSetNamespace,
			Resource: rbacv1.Role(roleName, kr.Namespace).
				WithRules(rules...).
				WithLabels(labels).
				WithAnnotations(annotations),
		})

		invocations = append(invocations, kubedef.Apply{
			Description:  fmt.Sprintf("%s: Role Binding", descriptionBase),
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
		invocations = append(invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Cluster Role", descriptionBase),
			Resource: rbacv1.ClusterRole(roleName).
				WithRules(rules...).
				WithLabels(labels).
				WithAnnotations(kubedef.BaseAnnotations()),
		})

		invocations = append(invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Cluster Role Binding", descriptionBase),
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

	return invocations
}

func makeRoles(req provisioning.StackRequest) (string, string) {
	roleName := fmt.Sprintf("foundation:managed:%s", kubedef.MakeDeploymentId(req.Focus.Server))
	roleBinding := fmt.Sprintf("foundation:managed:%s", kubedef.MakeDeploymentId(req.Focus.Server))

	return roleName, roleBinding
}

func (g GrantKubeACLs) serviceAccount(req provisioning.StackRequest) string {
	serviceAccount := g.ServiceAccount
	if serviceAccount == "" {
		serviceAccount = kubedef.MakeDeploymentId(req.Focus.Server)
	}
	return serviceAccount
}
