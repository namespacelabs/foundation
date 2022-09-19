// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package buildkit

import (
	"context"
	"fmt"

	"github.com/moby/buildkit/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/std/planning"
	"namespacelabs.dev/foundation/workspace/tasks"
)

func Prune(ctx context.Context, cfg planning.Configuration, targetPlatform specs.Platform) error {
	return tasks.Action("buildkit.prune").Run(ctx, func(ctx context.Context) error {
		cli, err := compute.GetValue(ctx, connectToClient(cfg, targetPlatform))
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
