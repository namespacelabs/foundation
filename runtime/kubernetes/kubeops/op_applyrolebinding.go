// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubeops

import (
	"context"
	"encoding/json"

	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubeblueprint"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
)

func registerApplyRoleBinding() {
	ops.RegisterFunc(func(ctx context.Context, d *schema.SerializedInvocation, spec *kubedef.OpApplyRoleBinding) (*ops.HandleResult, error) {
		ns, err := kubedef.InjectedKubeClusterNamespace(ctx)
		if err != nil {
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

		invocations := kubeblueprint.MakeInvocations(d.Description, scope, &kubetool.ContextualEnv{Namespace: ns.KubeConfig().Namespace},
			spec.RoleName, spec.RoleBindingName, makeMap(spec.Label), makeMap(spec.Annotation), spec.ServiceAccount, rules)

		res := &ops.HandleResult{}
		for _, spec := range invocations {
			d, spec, err := spec.ToDefinitionImpl(schema.PackageNames(d.Scope...)...)
			if err != nil {
				return nil, err
			}

			if h, err := apply(ctx, d.Description, schema.PackageNames(d.Scope...), spec); err != nil {
				return nil, err
			} else {
				res.Waiters = append(res.Waiters, h.Waiters...)
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
