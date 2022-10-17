// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package testing

import (
	"context"

	"namespacelabs.dev/foundation/build/registry"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/go-ids"
)

var (
	UseNamespaceCloud = false
)

func PrepareEnv(ctx context.Context, sourceEnv cfg.Context, ephemeral bool) cfg.Context {
	testInv := ids.NewRandomBase32ID(8)
	testEnv := &schema.Environment{
		Name:      "test-" + testInv,
		Purpose:   schema.Environment_TESTING,
		Runtime:   "kubernetes",
		Ephemeral: ephemeral,
	}

	newCfg := sourceEnv.Configuration().Derive(testEnv.Name, func(previous cfg.ConfigurationSlice) cfg.ConfigurationSlice {
		if UseNamespaceCloud {
			// Prepend as this configuration should take precedence.
			previous.Configuration = append(protos.WrapAnysOrDie(
				&registry.Provider{Provider: "nscloud"},
				&client.HostEnv{Provider: "nscloud"},
			), previous.Configuration...)
		}

		return previous
	})

	return cfg.MakeUnverifiedContext(newCfg, testEnv)
}
