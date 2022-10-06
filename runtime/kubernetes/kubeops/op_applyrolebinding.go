// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"

	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeblueprint"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	fnschema "namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/execution"
)

func registerApplyRoleBinding() {
	execution.Compile[*kubedef.OpApplyRoleBinding](func(ctx context.Context, inputs []*fnschema.SerializedInvocation) ([]*fnschema.SerializedInvocation, error) {
		ns, err := kubedef.InjectedKubeClusterNamespace(ctx)
		if err != nil {
			return nil, err
		}

		var res []*fnschema.SerializedInvocation
		for _, input := range inputs {
			spec := &kubedef.OpApplyRoleBinding{}
			if err := input.Impl.UnmarshalTo(spec); err != nil {
				return nil, err
			}

			scope := kubeblueprint.ClusterScope
			if spec.Namespaced {
				scope = kubeblueprint.NamespaceScope
			}

			var rules []*rbacv1.PolicyRuleApplyConfiguration
			if err := json.Unmarshal([]byte(spec.RulesJson), &rules); err != nil {
				return nil, fnerrors.InternalError("failed to unmarshal rules: %w", err)
			}

			invocations := kubeblueprint.MakeInvocations(input.Description, scope, &kubetool.ContextualEnv{Namespace: ns.KubeConfig().Namespace},
				spec.RoleName, spec.RoleBindingName, makeMap(spec.Label), makeMap(spec.Annotation), spec.ServiceAccount, rules)

			for _, inv := range invocations {
				compiled, err := inv.ToDefinition(fnschema.PackageNames(input.Scope...)...)
				if err != nil {
					return nil, err
				}
				res = append(res, compiled)
			}
		}

		return res, nil
	})
}

func makeMap(kvs []*kubedef.OpApplyRoleBinding_KeyValue) map[string]string {
	if len(kvs) == 0 {
		return nil
	}

	m := map[string]string{}
	for _, kv := range kvs {
		m[kv.Key] = kv.Value
	}
	return m
}
