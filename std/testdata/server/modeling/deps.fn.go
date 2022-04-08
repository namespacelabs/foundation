// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	"namespacelabs.dev/foundation/std/testdata/scopes"
	"namespacelabs.dev/foundation/std/testdata/scopes/data"
	"namespacelabs.dev/foundation/std/testdata/service/modeling"
)

type ServerDeps struct {
	modeling *modeling.ServiceDeps
}

func PrepareDeps(ctx context.Context) (server *ServerDeps, err error) {
	di := core.MakeInitializer()

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Instance:    "metricsSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &metrics.SingletonDeps{}
			var err error
			{
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Instance:    "tracingSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &tracing.SingletonDeps{}
			var err error
			{
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/scopes",
		Instance:    "scopes0",
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &scopes.ScopedDataDeps{}
			var err error
			{
				if deps.Data, err = data.ProvideData(ctx, "namespacelabs.dev/foundation/std/testdata/scopes", nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/modeling",
		Instance:    "modelingDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &modeling.ServiceDeps{}
			var err error
			{

				scopes0, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/scopes", "scopes0")
				if err != nil {
					return nil, err
				}
				if deps.One, err = scopes.ProvideScopedData(ctx, "namespacelabs.dev/foundation/std/testdata/service/modeling", nil, scopes0.(*scopes.ScopedDataDeps)); err != nil {
					return nil, err
				}
			}

			{

				scopes0, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/scopes", "scopes0")
				if err != nil {
					return nil, err
				}
				if deps.Two, err = scopes.ProvideScopedData(ctx, "namespacelabs.dev/foundation/std/testdata/service/modeling", nil, scopes0.(*scopes.ScopedDataDeps)); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			metricsSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", "metricsSingle")
			if err != nil {
				return err
			}
			return metrics.Prepare(ctx, metricsSingle.(*metrics.SingletonDeps))
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			tracingSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", "tracingSingle")
			if err != nil {
				return err
			}
			return tracing.Prepare(ctx, tracingSingle.(*tracing.SingletonDeps))
		},
	})

	server = &ServerDeps{}

	modelingDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/service/modeling", "modelingDeps")
	if err != nil {
		return nil, err
	}
	server.modeling = modelingDeps.(*modeling.ServiceDeps)

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	modeling.WireService(ctx, srv, server.modeling)
}
