// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
)

const (
	adminNamespace = "fn-admin"
)

// We use separate roles/accs to maintain a minimum set of permissions per usecase.
// This also removes the need to merge permissions on updates.
func adminRole(name string) string {
	return fmt.Sprintf("foundation:admin:%s-role", name)
}

func adminBinding(name string) string {
	return fmt.Sprintf("foundation:admin:%s-binding", name)
}

func adminServiceAccount(name string) string {
	return fmt.Sprintf("foundation-admin-%s-service-account", name)
}

func grantAdmin(ctx context.Context, env ops.Environment, admin *kubedef.OpAdmin) error {
	var rules []*applyrbacv1.PolicyRuleApplyConfiguration

	if err := json.Unmarshal([]byte(admin.RulesJson), &rules); err != nil {
		return err
	}

	if !validChars.MatchString(admin.Name) {
		return fmt.Errorf("Invalid admin name %q - it may only contain digits and lowercase letters.", admin.Name)
	}

	cfg, err := client.ConfigFromEnv(ctx, env)
	if err != nil {
		return err
	}

	cli, err := client.NewClientFromHostEnv(cfg)
	if err != nil {
		return err
	}

	if _, err := cli.CoreV1().Namespaces().Apply(ctx, applycorev1.Namespace(adminNamespace), kubedef.Ego()); err != nil {
		return err
	}

	if _, err := cli.CoreV1().ServiceAccounts(adminNamespace).Apply(ctx, applycorev1.ServiceAccount(adminServiceAccount(admin.Name), adminNamespace), kubedef.Ego()); err != nil {
		return err
	}

	if _, err := cli.RbacV1().ClusterRoles().Apply(ctx, applyrbacv1.ClusterRole(adminRole(admin.Name)).WithRules(rules...), kubedef.Ego()); err != nil {
		return err
	}

	if _, err := cli.RbacV1().ClusterRoleBindings().
		Apply(ctx, applyrbacv1.ClusterRoleBinding(adminBinding(admin.Name)).
			WithRoleRef(applyrbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(adminRole(admin.Name))).
			WithSubjects(applyrbacv1.Subject().
				WithKind("ServiceAccount").
				WithNamespace(adminNamespace).
				WithName(adminServiceAccount(admin.Name))), kubedef.Ego()); err != nil {
		return err
	}

	return nil
}
