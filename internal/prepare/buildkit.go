// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package prepare

import (
	"context"
	"fmt"

	"namespacelabs.dev/foundation/build/buildkit"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func PrepareBuildkit(env cfg.Context) compute.Computable[[]*schema.DevHost_ConfigureEnvironment] {
	return compute.Map(
		tasks.Action("prepare.buildkit").HumanReadablef("Preparing the buildkit daemon"),
		compute.Inputs().Indigestible("env", env),
		compute.Output{NotCacheable: true},
		func(ctx context.Context, _ compute.Resolved) ([]*schema.DevHost_ConfigureEnvironment, error) {
			containerName := buildkit.DefaultContainerName

			if conf, ok := buildkit.OverridesConfigType.CheckGet(env.Configuration()); ok {
				if conf.BuildkitAddr != "" {
					fmt.Fprintln(console.Stderr(ctx), "Buildkit has been manually configured, skipping setup.")
					return nil, nil
				}

				if conf.ContainerName != "" {
					containerName = conf.ContainerName
				}
			}

			_, err := buildkit.EnsureBuildkitd(ctx, containerName)
			return nil, err
		})
}
