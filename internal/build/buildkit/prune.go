// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildkit

import (
	"context"
	"fmt"

	"github.com/moby/buildkit/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func Prune(ctx context.Context, cfg cfg.Configuration, targetPlatform *specs.Platform) error {
	return tasks.Action("buildkit.prune").Run(ctx, func(ctx context.Context) error {
		cli, err := compute.GetValue(ctx, MakeClient(cfg, targetPlatform))
		if err != nil {
			return err
		}

		ch := make(chan client.UsageInfo)
		defer close(ch)

		log := console.TypedOutput(ctx, "buildkit", console.CatOutputTool)

		go func() {
			for du := range ch {
				fmt.Fprintln(log, du)
			}
		}()

		return cli.Prune(ctx, ch)
	})
}
