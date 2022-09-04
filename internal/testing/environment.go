// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/vcluster"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/go-ids"
)

var (
	UseVClusters      = false
	UseNamespaceCloud = false
)

func PrepareEnv(ctx context.Context, sourceEnv planning.Context, ephemeral bool) planning.Context {
	testInv := ids.NewRandomBase32ID(8)
	testEnv := &schema.Environment{
		Name:      "test-" + testInv,
		Purpose:   schema.Environment_TESTING,
		Runtime:   "kubernetes",
		Ephemeral: ephemeral,
	}

	devHost := &schema.DevHost{
		Configure:         devhost.ConfigurationForEnv(sourceEnv).WithoutConstraints(),
		ConfigurePlatform: sourceEnv.DevHost().ConfigurePlatform,
	}

	if UseNamespaceCloud {
		devHost.Configure = []*schema.DevHost_ConfigureEnvironment{
			{Configuration: protos.WrapAnysOrDie(
				&registry.Provider{Provider: "nscloud"},
				&client.HostEnv{Provider: "nscloud"},
			)},
		}
	}

	return planning.MakeUnverifiedContext(sourceEnv.Workspace(), sourceEnv.WorkspaceLoadedFrom(), devHost, testEnv, sourceEnv.ErrorLocation())
}

func makeDeleteEnv(env runtime.Selector) func(context.Context) error {
	return func(ctx context.Context) error {
		// This always works because the vcluster is also deployed to the same namespace.
		if _, err := runtime.For(ctx, env).DeleteRecursively(ctx, false); err != nil {
			return err
		}

		return nil
	}
}

func envWithVCluster(ctx context.Context, sourceEnv planning.Context, vcluster *vcluster.VCluster) (planning.Context, func(context.Context) error, error) {
	testEnv := sourceEnv.Environment()
	devHost := sourceEnv.DevHost()

	conn, err := vcluster.Access(ctx)
	if err != nil {
		return nil, nil, err
	}

	c, err := devhost.MakeConfiguration(conn.HostEnv())
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	// Make sure we look up the new configuration first.
	configure := []*schema.DevHost_ConfigureEnvironment{c}
	configure = append(configure, devHost.Configure...)

	env := planning.MakeUnverifiedContext(sourceEnv.Workspace(), sourceEnv.WorkspaceLoadedFrom(), &schema.DevHost{
		Configure:         configure,
		ConfigurePlatform: devHost.ConfigurePlatform,
	}, testEnv, sourceEnv.ErrorLocation())

	deleteEnv := makeDeleteEnv(sourceEnv)
	return env, func(ctx context.Context) error {
		err0 := conn.Close()

		if err1 := deleteEnv(ctx); err1 != nil {
			return err1
		}

		return err0
	}, nil
}
