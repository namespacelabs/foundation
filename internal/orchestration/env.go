// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubedef"
	"namespacelabs.dev/foundation/runtime/kubernetes/kubetool"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
)

func makeOrchEnv(ctx planning.Context) planning.Context {
	// We use a static environment here, since the orchestrator has global scope.
	env := &schema.Environment{
		Name:      kubedef.AdminNamespace,
		Runtime:   ctx.Environment().Runtime,
		Ephemeral: false,

		// TODO - this can't be empty, since std/runtime/kubernetes/extension.cue checks it.
		Purpose: schema.Environment_PRODUCTION,
	}

	// It is imperative that the original environment name is used as the key of
	// the derived configuration, or else we create a completely separate set of
	// resources for the admin environment. For example, with nscloud, we'd
	// create a cluster for fn-admin, and another one for the actual workloads.
	originalEnv := ctx.Environment().Name

	cfg := ctx.Configuration().Derive(originalEnv, func(previous planning.ConfigurationSlice) planning.ConfigurationSlice {
		previous.Configuration = append(previous.Configuration, protos.WrapAnyOrDie(
			&kubetool.KubernetesEnv{Namespace: kubedef.AdminNamespace}, // pin deployments to admin namespace
		))
		return previous
	})

	return planning.MakeUnverifiedContext(cfg, ctx.Workspace(), env, ctx.ErrorLocation())
}
