// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
)

type RunOpts struct {
	PackageName          string
	RegisterInitializers func(*core.DependencyGraph)
	WireServices         func(context.Context, Server, core.Dependencies) []error
}

func Run(ctx context.Context, opts RunOpts, listenOpts ListenOpts) {
	ctx = core.ZLog.WithContext(ctx)

	resources := core.PrepareEnv(opts.PackageName)
	defer resources.Close(ctx)

	ctx = core.WithResources(ctx, resources)

	depgraph := core.NewDependencyGraph()
	opts.RegisterInitializers(depgraph)
	if err := depgraph.RunInitializers(ctx); err != nil {
		core.ZLog.Fatal().Err(err).Send()
	}

	core.InitializationDone()

	if err := Listen(ctx, listenOpts, func(srv Server) {
		if errs := opts.WireServices(ctx, srv, depgraph); len(errs) > 0 {
			core.ZLog.Fatal().Errs("errors", errs).Msgf("%d services failed to initialize.", len(errs))
		}
	}); err != nil {
		core.ZLog.Fatal().Err(err).Msgf("failed to listen.")
	}
}
