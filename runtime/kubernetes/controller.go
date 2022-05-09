// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

const (
	clusterRole        = "foundation:controller:testing-controller"
	clusterRoleBinding = "foundation:controller:testing-controller-binding"
	serviceAccount     = "foundation-testing-controller-service-account"
)

func (r k8sRuntime) RunController(ctx context.Context, runOpts runtime.ServerRunOpts) error {
	cli, err := client.NewClientFromHostEnv(r.hostEnv)
	if err != nil {
		return err
	}

	// We intentionally don't use r.ns() since we don't want one controller per workspace.
	// Controllers operate with cluster-wide permissions.
	ns := r.env.Name

	// TODO add annotations/labels?
	if _, err := cli.CoreV1().Namespaces().Apply(ctx, applycorev1.Namespace(ns), kubedef.Ego()); err != nil {
		return err
	}

	acc := applycorev1.ServiceAccount(serviceAccount, ns)
	if _, err := cli.CoreV1().ServiceAccounts(ns).Apply(ctx, acc, kubedef.Ego()); err != nil {
		return err
	}

	role := applyrbacv1.ClusterRole(clusterRole).WithRules(
		applyrbacv1.PolicyRule().WithAPIGroups("").WithResources("namespaces").WithVerbs("list", "delete"),
		applyrbacv1.PolicyRule().WithAPIGroups("").WithResources("events").WithVerbs("list"),
	)
	if _, err := cli.RbacV1().ClusterRoles().Apply(ctx, role, kubedef.Ego()); err != nil {
		return err
	}

	binding := applyrbacv1.ClusterRoleBinding(clusterRoleBinding).
		WithSubjects(applyrbacv1.Subject().WithKind("ServiceAccount").WithName(serviceAccount).WithNamespace(ns)).
		WithRoleRef(applyrbacv1.RoleRef().WithKind("ClusterRole").WithName(clusterRole))
	if _, err := cli.RbacV1().ClusterRoleBindings().Apply(ctx, binding, kubedef.Ego()); err != nil {
		return err
	}

	name := fmt.Sprintf("controller-%v", labelName(runOpts.Command))
	container := applycorev1.Container().
		WithName(name).
		WithImage(runOpts.Image.RepoAndDigest()).
		WithArgs(runOpts.Args...).
		WithCommand(runOpts.Command...).
		WithSecurityContext(
			applycorev1.SecurityContext().
				WithReadOnlyRootFilesystem(runOpts.ReadOnlyFilesystem))

	pod := applycorev1.Pod(name, ns).WithSpec(applycorev1.PodSpec().
		WithContainers(container).
		WithRestartPolicy(corev1.RestartPolicyOnFailure).
		WithServiceAccountName(serviceAccount))

	// Shall we block on the controller becoming healthy?
	_, err = cli.CoreV1().Pods(ns).Apply(ctx, pod, kubedef.Ego())
	return err

}
