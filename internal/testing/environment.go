// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package testing

import (
	"context"

	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/build/registry"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/providers/nscloud"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/go-ids"
)

var (
	UseNamespaceCloud        = false
	UseNamespaceBuildCluster = false
)

func PrepareEnv(ctx context.Context, sourceEnv cfg.Context, ephemeral bool) (cfg.Context, error) {
	testInv := ids.NewRandomBase32ID(8)
	testEnv := &schema.Environment{
		Name:      "test-" + testInv,
		Purpose:   schema.Environment_TESTING,
		Runtime:   "kubernetes",
		Ephemeral: ephemeral,
	}

	var messages []*anypb.Any
	if UseNamespaceBuildCluster {
		msg, err := nscloud.EnsureBuildCluster(ctx, api.Methods)
		if err != nil {
			return nil, err
		}
		messages = append(messages, protos.WrapAnyOrDie(msg))
	}

	newCfg := sourceEnv.Configuration().Derive(testEnv.Name, func(previous cfg.ConfigurationSlice) cfg.ConfigurationSlice {
		if UseNamespaceCloud {
			messages = append(messages, protos.WrapAnysOrDie(
				&client.HostEnv{Provider: "nscloud"},
				&registry.Provider{Provider: "nscloud"},
			)...)
		}

		// Prepend as the testing configurations should take precedence.
		return cfg.ConfigurationSlice{Configuration: append(messages, previous.Configuration...)}
	})

	return cfg.MakeUnverifiedContext(newCfg, testEnv), nil
}
