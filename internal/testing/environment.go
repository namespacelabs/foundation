// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/go-ids"
)

var (
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

	newCfg := sourceEnv.Configuration().Derive(testEnv.Name, func(previous planning.ConfigurationSlice) planning.ConfigurationSlice {
		if UseNamespaceCloud {
			return planning.ConfigurationSlice{
				Configuration: protos.WrapAnysOrDie(
					&registry.Provider{Provider: "nscloud"},
					&client.HostEnv{Provider: "nscloud"},
				),
			}
		}

		return previous
	})

	return planning.MakeUnverifiedContext(newCfg, sourceEnv.Workspace(), testEnv, sourceEnv.ErrorLocation())
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
