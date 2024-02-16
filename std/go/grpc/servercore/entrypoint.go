// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"namespacelabs.dev/foundation/std/go/core"
)

var (
	metric_initialized = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "ns",
		Subsystem: "gogrpc",
		Name:      "server_initialized",
	}, []string{"package_name", "revision"})
)

func init() {
	prometheus.MustRegister(
		metric_initialized,
	)
}

type RunOpts struct {
	PackageName          string
	RegisterInitializers func(*core.DependencyGraph)
	WireServices         func(context.Context, Server, core.Dependencies) []error
}

func Run(ctx context.Context, opts RunOpts, listenOpts ListenOpts) {
	ctx = core.ZLog.WithContext(ctx)

	resources, rev := core.PrepareEnv(opts.PackageName)
	defer resources.Close(ctx)

	ctx = core.WithResources(ctx, resources)

	depgraph := core.NewDependencyGraph()
	opts.RegisterInitializers(depgraph)
	if err := depgraph.RunInitializers(ctx); err != nil {
		core.ZLog.Fatal().Err(err).Send()
	}

	core.InitializationDone()

	metric_initialized.WithLabelValues(opts.PackageName, rev).Inc()

	if err := Listen(ctx, listenOpts, func(srv Server) {
		if errs := opts.WireServices(ctx, srv, depgraph); len(errs) > 0 {
			core.ZLog.Fatal().Errs("errors", errs).Msgf("%d services failed to initialize.", len(errs))
		}
	}); err != nil {
		core.ZLog.Fatal().Err(err).Msgf("failed to listen.")
	}
}
