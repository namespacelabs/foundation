// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/schema"
)

const (
	adminNamespace = "fn-admin"
	ctrlPackage    = "namespacelabs.dev/foundation/std/runtime/kubernetes/controller"
)

// We use separate roles/accs to maintain a minimum set of permissions per usecase.
// This also removes the need to merge permissions on updates.
func adminRole(name string) string {
	return fmt.Sprintf("foundation:admin:%s-role", name)
}

func adminBinding(name string) string {
	return fmt.Sprintf("foundation:admin:%s-binding", name)
}

func isController(pkg schema.PackageName) bool {
	return strings.HasPrefix(pkg.String(), ctrlPackage)
}

func grantAdmin(ctx context.Context, env ops.Environment, scope []schema.PackageName, admin *kubedef.OpAdmin) error {
	if !validChars.MatchString(admin.ServiceAccount) {
		return fnerrors.InternalError("Invalid service account name %q - it may only contain digits and lowercase letters.", admin.ServiceAccount)
	}

	for _, s := range scope {
		if !isController(s) {
			return fnerrors.InternalError("%s: only kubernetes controllers are allowed to request admin rights", s)
		}
	}

	var rules []*applyrbacv1.PolicyRuleApplyConfiguration

	if err := json.Unmarshal([]byte(admin.RulesJson), &rules); err != nil {
		return err
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

	if _, err := cli.CoreV1().ServiceAccounts(adminNamespace).Apply(ctx, applycorev1.ServiceAccount(admin.ServiceAccount, adminNamespace), kubedef.Ego()); err != nil {
		return err
	}

	if _, err := cli.RbacV1().ClusterRoles().Apply(ctx, applyrbacv1.ClusterRole(adminRole(admin.ServiceAccount)).WithRules(rules...), kubedef.Ego()); err != nil {
		return err
	}

	if _, err := cli.RbacV1().ClusterRoleBindings().
		Apply(ctx, applyrbacv1.ClusterRoleBinding(adminBinding(admin.ServiceAccount)).
			WithRoleRef(applyrbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(adminRole(admin.ServiceAccount))).
			WithSubjects(applyrbacv1.Subject().
				WithKind("ServiceAccount").
				WithNamespace(adminNamespace).
				WithName(admin.ServiceAccount)), kubedef.Ego()); err != nil {
		return err
	}

	return nil
}
