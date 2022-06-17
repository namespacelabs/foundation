// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"

	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/provision"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/vcluster"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/devhost"
	"namespacelabs.dev/go-ids"
)

var UseVClusters = false

func PrepareBuildEnv(ctx context.Context, sourceEnv provision.Env, ephemeral bool) provision.Env {
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

	env := provision.MakeEnv(&workspace.Root{
		Workspace:     sourceEnv.Root().Workspace,
		WorkspaceData: sourceEnv.Root().WorkspaceData,
		DevHost:       devHost,
	}, testEnv)

	return env
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

func envWithVCluster(ctx context.Context, sourceEnv ops.Environment, vcluster *vcluster.VCluster) (provision.Env, func(context.Context) error, error) {
	testEnv := sourceEnv.Proto()
	devHost := sourceEnv.DevHost()

	conn, err := vcluster.Access(ctx)
	if err != nil {
		return provision.Env{}, nil, err
	}

	c, err := devhost.MakeConfiguration(conn.HostEnv())
	if err != nil {
		conn.Close()
		return provision.Env{}, nil, err
	}

	// Make sure we look up the new configuration first.
	configure := []*schema.DevHost_ConfigureEnvironment{c}
	configure = append(configure, devHost.Configure...)

	env := provision.MakeEnv(&workspace.Root{
		Workspace: sourceEnv.Workspace(),
		DevHost: &schema.DevHost{
			Configure:         configure,
			ConfigurePlatform: devHost.ConfigurePlatform,
		},
	}, testEnv)

	deleteEnv := makeDeleteEnv(sourceEnv)
	return env, func(ctx context.Context) error {
		err0 := conn.Close()

		if err1 := deleteEnv(ctx); err1 != nil {
			return err1
		}

		return err0
	}, nil
}
