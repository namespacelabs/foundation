// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package prepare

import (
	"context"

	"namespacelabs.dev/foundation/internal/runtime/kubernetes"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/schema/orchestration"
	"namespacelabs.dev/foundation/std/cfg"
)

type Stage struct {
	Pre func(ch chan *orchestration.Event)
	Run func(ctx context.Context, env cfg.Context, ch chan *orchestration.Event) (*schema.DevHost_ConfigureEnvironment, error)
}

type ClusterStage struct {
	Pre  func(ch chan *orchestration.Event)
	Post func(ch chan *orchestration.Event)
	Run  func(ctx context.Context, env cfg.Context, devhost *schema.DevHost_ConfigureEnvironment, kube *kubernetes.Cluster, ch chan *orchestration.Event) error
}
