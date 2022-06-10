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

type GrantKubeACLs struct {
	DescriptionBase string
	ServiceAccount  string
	ClusterRole     string
	Rules           *rbacv1.PolicyRuleApplyConfiguration
}

func (g GrantKubeACLs) Compile(req configure.StackRequest, out *configure.ApplyOutput) error {
	clusterRoleBinding := fmt.Sprintf("fn-%s-%s", g.ServiceAccount, g.ClusterRole)

	serviceAccount := g.ServiceAccount
	if serviceAccount == "" {
		serviceAccount = kubedef.MakeDeploymentId(req.Focus.Server)
	}

	clusterRole := g.ClusterRole
	if clusterRole == "" {
		clusterRole = fmt.Sprintf("foundation:managed:%s", kubedef.MakeDeploymentId(req.Focus.Server))
	}

	if g.ServiceAccount == "" || g.ClusterRole == "" {
		clusterRoleBinding = fmt.Sprintf("foundation:managed:%s", kubedef.MakeDeploymentId(req.Focus.Server))
	}

	if g.Rules == nil {
		return fnerrors.BadInputError("Rules is required")
	}

	namespace := kubetool.FromRequest(req).Namespace
	labels := kubedef.MakeLabels(req.Env, req.Focus.Server)

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: fmt.Sprintf("%s: Service Account", g.DescriptionBase),
		Resource:    corev1.ServiceAccount(serviceAccount, namespace).WithLabels(labels),
	})

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: fmt.Sprintf("%s: Cluster Role", g.DescriptionBase),
		Resource:    rbacv1.ClusterRole(clusterRole).WithRules(g.Rules).WithLabels(labels),
	})

	out.Invocations = append(out.Invocations, kubedef.Apply{
		Description: fmt.Sprintf("%s: Cluster Role Binding", g.DescriptionBase),
		Resource: rbacv1.ClusterRoleBinding(clusterRoleBinding).
			WithLabels(labels).
			WithRoleRef(rbacv1.RoleRef().
				WithAPIGroup("rbac.authorization.k8s.io").
				WithKind("ClusterRole").
				WithName(clusterRole)).
			WithSubjects(rbacv1.Subject().
				WithKind("ServiceAccount").
				WithNamespace(namespace).
				WithName(serviceAccount)),
	})

	out.Extensions = append(out.Extensions, kubedef.ExtendSpec{
		With: &kubedef.SpecExtension{
			ServiceAccount: serviceAccount,
		},
	})

	return nil
}
