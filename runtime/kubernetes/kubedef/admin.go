// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubedef

import (
	"fmt"
	"regexp"

	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	applyrbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/controller"
	"namespacelabs.dev/foundation/schema"
)

const (
	AdminNamespace = "fn-admin"
)

var (
	validChars = regexp.MustCompile("[a-z0-9-]+")
)

// We use separate roles/accs to maintain a minimum set of permissions per usecase.
// This also removes the need to merge permissions on updates.
func adminRole(name string) string {
	return fmt.Sprintf("foundation:admin:%s-role", name)
}

func adminBinding(name string) string {
	return fmt.Sprintf("foundation:admin:%s-binding", name)
}

// Only a limited set of nodes is allowed to set this.
type Admin struct {
	Description    string
	ServiceAccount string
	Rules          []*applyrbacv1.PolicyRuleApplyConfiguration
}

func (a Admin) ToDefinition(scope ...schema.PackageName) ([]*schema.Definition, error) {
	if len(a.Rules) == 0 {
		return nil, fnerrors.InternalError("no admin rules specified")
	}
	if !validChars.MatchString(a.ServiceAccount) {
		return nil, fnerrors.InternalError("Invalid service account name %q - it may only contain digits and lowercase letters.", a.ServiceAccount)
	}

	for _, s := range scope {
		if !controller.IsController(s) {
			return nil, fnerrors.InternalError("%s: only kubernetes controllers are allowed to request admin rights", s)
		}
	}

	applies := []Apply{{
		Description: "Admin Namespace",
		Resource:    "namespaces",
		Name:        AdminNamespace,
		Body:        applycorev1.Namespace(AdminNamespace),
	}, {
		Description: "Admin Service Account",
		Resource:    "serviceaccounts",
		Namespace:   AdminNamespace,
		Name:        a.ServiceAccount,
		Body:        applycorev1.ServiceAccount(a.ServiceAccount, AdminNamespace),
	}, {
		Description: "Admin Cluster Role",
		Resource:    "clusterroles",
		Name:        adminRole(a.ServiceAccount),
		Body:        applyrbacv1.ClusterRole(adminRole(a.ServiceAccount)).WithRules(a.Rules...),
	}, {
		Description: "Admin Cluster Role Binding",
		Resource:    "clusterrolebindings",
		Name:        adminBinding(a.ServiceAccount),
		Body: applyrbacv1.ClusterRoleBinding(adminBinding(a.ServiceAccount)).
			WithRoleRef(applyrbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(adminRole(a.ServiceAccount))).
			WithSubjects(applyrbacv1.Subject().
				WithKind("ServiceAccount").
				WithNamespace(AdminNamespace).
				WithName(a.ServiceAccount)),
	}}

	var defs []*schema.Definition
	for _, a := range applies {
		def, err := a.ToDefinition(scope...)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def...)
	}
	return defs, nil
}
