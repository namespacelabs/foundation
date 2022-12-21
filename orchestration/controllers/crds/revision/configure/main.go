// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	_ "embed"
	"fmt"

	rbacv1 "k8s.io/client-go/applyconfigurations/rbac/v1"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeblueprint"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubeparser"
	"namespacelabs.dev/foundation/framework/provisioning"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
)

//go:embed revisioncrd.yaml
var revisionCrd string

func main() {
	h := provisioning.NewHandlers()
	henv := h.MatchEnv(&schema.Environment{Runtime: "kubernetes"})
	henv.HandleStack(configuration{})
	provisioning.Handle(h)
}

type configuration struct{}

func (configuration) Apply(ctx context.Context, req provisioning.StackRequest, out *provisioning.ApplyOutput) error {
	serviceAccount := makeServiceAccount(req.Focus.Server)

	apply, err := kubeparser.Single([]byte(revisionCrd))
	if err != nil {
		return fnerrors.InternalError("failed to parse the HTTP gRPC Transcoder CRD: %w", err)
	}

	out.Invocations = append(out.Invocations, kubedef.Create{
		Description:      "Revision CustomResourceDefinition",
		Resource:         "customresourcedefinitions",
		Body:             apply.Resource,
		UpdateIfExisting: true,
	})

	grant := kubeblueprint.GrantKubeACLs{
		DescriptionBase: "Revision",
		ServiceAccount:  serviceAccount,
	}

	grant.Rules = append(grant.Rules, rbacv1.PolicyRule().
		WithAPIGroups("k8s.namespacelabs.dev").
		WithResources("revisions", "revisions/status").
		WithVerbs("get", "list", "watch", "create", "update", "delete", "patch"))

	// We leverage `record.EventRecorder` from "k8s.io/client-go/tools/record" which
	// creates `Event` objects with the API group "". This rule ensures that
	// the event objects created by the controller are accepted by the k8s API server.
	grant.Rules = append(grant.Rules, rbacv1.PolicyRule().
		WithAPIGroups("").
		WithResources("events").
		WithVerbs("create"))

	if err := grant.Compile(req, kubeblueprint.NamespaceScope, out); err != nil {
		return err
	}

	return nil
}

func (configuration) Delete(context.Context, provisioning.StackRequest, *provisioning.DeleteOutput) error {
	// XXX unimplemented
	return nil
}

// TODO: duplicate from orchestration/server/tool/main.go
func makeServiceAccount(srv runtime.Deployable) string {
	return fmt.Sprintf("admin-%s", kubedef.MakeDeploymentId(srv))
}
