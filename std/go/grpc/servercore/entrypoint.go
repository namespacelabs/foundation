// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package servercore

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"namespacelabs.dev/foundation/std/go/core"
)

var (
	serverInitializedInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "ns",
		Subsystem: "gogrpc",
		Name:      "server_initialized_info",
	}, []string{"package_name", "revision"})

	serverInitializedTimestamp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "ns",
		Subsystem: "gogrpc",
		Name:      "server_initialized_timestamp_seconds",
	}, []string{"package_name", "revision"})

	serverCertValidity = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "ns",
		Subsystem: "gogrpc",
		Name:      "certificate_validity_not_after_timestamp_seconds",
	}, []string{"common_name"})

	secretChecksumInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "ns",
		Subsystem: "gogrpc",
		Name:      "secret_checksum_info",
	}, []string{"secret_ref", "checksum"})
)

func init() {
	prometheus.MustRegister(
		serverInitializedInfo,
		serverInitializedTimestamp,
		serverCertValidity,
		secretChecksumInfo,
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

	serverInitializedInfo.WithLabelValues(opts.PackageName, rev).Inc()
	serverInitializedTimestamp.WithLabelValues(opts.PackageName, rev).Set(float64(time.Now().Unix()))

	if err := Listen(ctx, listenOpts, func(srv Server) {
		if errs := opts.WireServices(ctx, srv, depgraph); len(errs) > 0 {
			core.ZLog.Fatal().Errs("errors", errs).Msgf("%d services failed to initialize.", len(errs))
		}

		if err := depgraph.RunPostInitializers(ctx); err != nil {
			core.ZLog.Fatal().Err(err).Send()
		}
	}); err != nil {
		core.ZLog.Fatal().Err(err).Msgf("failed to listen.")
	}
}
