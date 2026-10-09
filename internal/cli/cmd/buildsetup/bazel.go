// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package buildsetup

import (
	"context"
	"os"

	"namespacelabs.dev/foundation/internal/build/binary"
	"namespacelabs.dev/foundation/internal/cli/cmd/cluster"
	"namespacelabs.dev/foundation/internal/compute"
	golangintegration "namespacelabs.dev/foundation/internal/integrations/golang"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
)

func ConfigureGoBuilder(ctx context.Context, env cfg.Context, clusterName string, static bool) error {
	if golangintegration.GoBuilderKind.Get(env.Configuration()) != golangintegration.GoBuilderMaybeBazel {
		return nil
	}

	bazelrc, err := os.CreateTemp("", "nsdev-bazel-*.bazelrc")
	if err != nil {
		return err
	}
	bazelrcPath := bazelrc.Name()
	compute.On(ctx).Cleanup(tasks.Action("bazel.cleanup-config"), func(context.Context) error {
		return os.Remove(bazelrcPath)
	})
	if err := bazelrc.Close(); err != nil {
		return err
	}

	if err := cluster.SetupBazelRemoteExecution(ctx, bazelrcPath, clusterName, static); err != nil {
		return err
	}

	builder := golangintegration.MaybeBazelBuilder(bazelrcPath, env.Workspace().LoadedFrom().AbsPath)
	binary.BuildGo = builder.GoBuilder
	golangintegration.ConfigureBuilder(builder)
	return nil
}
