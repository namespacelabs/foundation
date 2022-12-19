// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"path/filepath"

	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/planning/tool/protocol"
	"namespacelabs.dev/foundation/internal/support/naming"
	"namespacelabs.dev/foundation/library/kubernetes/rbac"
	"namespacelabs.dev/foundation/schema"
)

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleApply(func(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
		intent := &rbac.ClusterRoleIntent{}
		if err := req.UnpackInput(intent); err != nil {
			return err
		}

		source := &protocol.ResourceInstance{}
		if err := req.UnpackInput(source); err != nil {
			return err
		}

		roleName := "ns:user:" + naming.DomainFragLikeN("-", filepath.Base(source.ResourceInstance.PackageName), source.ResourceInstance.Name, naming.StableIDN(source.ResourceInstanceId, 8))
		labels := map[string]string{}

		clusterRole := rbacv1.ClusterRole(roleName).
			WithLabels(labels).
			WithAnnotations(kubedef.BaseAnnotations())

		for _, rule := range intent.Rules {
			r := rbacv1.PolicyRule().WithAPIGroups(rule.ApiGroups...).WithResources(rule.Resources...).WithVerbs(rule.Verbs...)
			clusterRole = clusterRole.WithRules(r)
		}

		out.Invocations = append(out.Invocations, kubedef.Apply{
			Description: fmt.Sprintf("%s: Cluster Role", intent.Name),
			Resource:    clusterRole,
		})

		out.OutputResourceInstance = &rbac.ClusterRoleInstance{
			Name: roleName,
		}
		return nil
	})
	provisioning.Handle(h)
}
