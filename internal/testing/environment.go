// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/runtime/kubernetes/vcluster"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
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

	// XXX update configuration envkey.
	newCfg := sourceEnv.Configuration().Derive(func(previous []*anypb.Any) []*anypb.Any {
		if UseNamespaceCloud {
			return protos.WrapAnysOrDie(
				&registry.Provider{Provider: "nscloud"},
				&client.HostEnv{Provider: "nscloud"},
			)
		}

		return previous
	})

	return planning.MakeUnverifiedContext(newCfg, sourceEnv.Workspace(), sourceEnv.WorkspaceLoadedFrom(), testEnv, sourceEnv.ErrorLocation())
}

func makeDeleteEnv(env planning.Context) func(context.Context) error {
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
	cfg := sourceEnv.Configuration().Derive(func(previous []*anypb.Any) []*anypb.Any {
		return append(c.Configuration, previous...)
	})

	env := planning.MakeUnverifiedContext(cfg, sourceEnv.Workspace(), sourceEnv.WorkspaceLoadedFrom(), testEnv, sourceEnv.ErrorLocation())

	deleteEnv := makeDeleteEnv(sourceEnv)
	return env, func(ctx context.Context) error {
		err0 := conn.Close()

		if err1 := deleteEnv(ctx); err1 != nil {
			return err1
		}

		return err0
	}, nil
}
