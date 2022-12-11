// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/parsing/devhost"
	"namespacelabs.dev/foundation/internal/runtime/kubernetes/client"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
)

func PrepareExistingK8s(env cfg.Context, kubeConfig, contextName string, registry proto.Message) Stage {
	return Stage{
		Run: func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error) {
			var confs []proto.Message
			hostEnv := &client.HostEnv{
				Kubeconfig: kubeConfig,
				Context:    contextName,
			}
			confs = append(confs, hostEnv)
			if registry != nil {
				confs = append(confs, registry)
			}

			ch <- &orchestration.Event{
				Category:   "Cluster",
				ResourceId: "existing",
				Scope:      fmt.Sprintf("Configure existing context %q", contextName),
				Ready:      orchestration.Event_READY,
				Stage:      orchestration.Event_DONE,
			}

			return devhost.MakeConfiguration(confs...)
		},
	}
}
