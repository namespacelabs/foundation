// This file was automatically generated.
package main

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
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
	di := fninit.MakeInitializer()

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &metrics.ExtensionDeps{}
			var err error
			{
				caller := cf.ForInstance("Interceptors")
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &tracing.ExtensionDeps{}
			var err error
			{
				caller := cf.ForInstance("Interceptors")
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/scopes",
		Typename:    "ScopedDataDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &scopes.ScopedDataDeps{}
			var err error
			{
				caller := cf.ForInstance("Data")
				if deps.Data, err = data.ProvideData(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/modeling",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &modeling.ServiceDeps{}
			var err error
			{
				caller := cf.ForInstance("One")
				scopedDataDeps, err := di.Get(ctx, caller, "namespacelabs.dev/foundation/std/testdata/scopes", "ScopedDataDeps")
				if err != nil {
					return nil, err
				}
				if deps.One, err = scopes.ProvideScopedData(ctx, caller, nil, scopedDataDeps.(*scopes.ScopedDataDeps)); err != nil {
					return nil, err
				}
			}

			{
				caller := cf.ForInstance("Two")
				scopedDataDeps, err := di.Get(ctx, caller, "namespacelabs.dev/foundation/std/testdata/scopes", "ScopedDataDeps")
				if err != nil {
					return nil, err
				}
				if deps.Two, err = scopes.ProvideScopedData(ctx, caller, nil, scopedDataDeps.(*scopes.ScopedDataDeps)); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.AddInitializer(fninit.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", "ExtensionDeps")
			if err != nil {
				return err
			}
			return metrics.Prepare(ctx, extensionDeps.(*metrics.ExtensionDeps))
		},
	})

	di.AddInitializer(fninit.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", "ExtensionDeps")
			if err != nil {
				return err
			}
			return tracing.Prepare(ctx, extensionDeps.(*tracing.ExtensionDeps))
		},
	})

	server = &ServerDeps{}

	modelingDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/testdata/service/modeling", "ServiceDeps")
	if err != nil {
		return nil, err
	}
	server.modeling = modelingDeps.(*modeling.ServiceDeps)

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	modeling.WireService(ctx, srv, server.modeling)
}
