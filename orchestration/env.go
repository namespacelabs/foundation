// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package orchestration

import (
	"context"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/framework/kubernetes/kubedef"
	"namespacelabs.dev/foundation/framework/kubernetes/kubetool"
	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
)

const (
	serverPkg schema.PackageName = "namespacelabs.dev/foundation/orchestration/server"
	toolPkg   schema.PackageName = "namespacelabs.dev/foundation/orchestration/server/tool"
)

var UsePinnedOrchestrator = true

func MakeSyntheticConfiguration(wsproto *schema.Workspace, envName string, hostEnv *client.HostEnv, extra ...proto.Message) cfg.Configuration {
	messages := []proto.Message{hostEnv}
	messages = append(messages, extra...)

	ws := cfg.MakeWorkspace(wsproto, nil)

	return cfg.MakeConfigurationWith(envName, ws, cfg.ConfigurationSlice{Configuration: protos.WrapAnysOrDie(messages...)})
}

func MakeSyntheticContext(wsproto *schema.Workspace, env *schema.Environment, hostEnv *client.HostEnv, extra ...proto.Message) cfg.Context {
	newCfg := MakeSyntheticConfiguration(wsproto, env.Name, hostEnv, extra...)
	return cfg.MakeUnverifiedContext(newCfg, env)
}

func MakeOrchestratorContext(ctx context.Context, conf cfg.Configuration) (cfg.Context, error) {
	// We use a static environment here, since the orchestrator has global scope.
	envProto := &schema.Environment{
		Name:      kubedef.AdminNamespace,
		Runtime:   "kubernetes", // XXX should be an input.
		Ephemeral: false,

		// TODO - this can't be empty, since std/runtime/kubernetes/extension.cue checks it.
		Purpose: schema.Environment_PRODUCTION,
	}

	var prebuilts []*schema.Workspace_BinaryDigest

	if UsePinnedOrchestrator {
		res, err := fnapi.GetLatestPrebuilts(ctx, serverPkg, toolPkg)
		if err != nil {
			return nil, err
		}

		for _, prebuilt := range res.Prebuilt {
			prebuilts = append(prebuilts, &schema.Workspace_BinaryDigest{
				PackageName: prebuilt.PackageName,
				Repository:  prebuilt.Repository,
				Digest:      prebuilt.Digest,
			})
		}
	}

	newCfg := conf.Derive(kubedef.AdminNamespace, func(previous cfg.ConfigurationSlice) cfg.ConfigurationSlice {
		previous.Configuration = append(previous.Configuration, protos.WrapAnysOrDie(
			&kubetool.KubernetesEnv{Namespace: kubedef.AdminNamespace}, // pin deployments to admin namespace
			&binary.Prebuilts{PrebuiltBinary: prebuilts},               // TODO: prebuilt overwrites are internal for now. Consider merging if these become more frequently used.
		)...)
		return previous
	})

	return cfg.MakeUnverifiedContext(newCfg, envProto), nil
}
