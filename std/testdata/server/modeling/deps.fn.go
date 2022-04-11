// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/schema"
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

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &metrics.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Interceptors").WithContext(ctx)
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &tracing.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Interceptors").WithContext(ctx)
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/scopes",
		Typename:    "ScopedDataDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &scopes.ScopedDataDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Data").WithContext(ctx)
				if deps.Data, err = data.ProvideData(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/modeling",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &modeling.ServiceDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "One").WithContext(ctx)
				scopedDataDeps, err := di.Get(ctx,
					"namespacelabs.dev/foundation/std/testdata/scopes", "ScopedDataDeps")
				if err != nil {
					return nil, err
				}
				if deps.One, err = scopes.ProvideScopedData(ctx, nil, scopedDataDeps.(*scopes.ScopedDataDeps)); err != nil {
					return nil, err
				}
			}

			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Two").WithContext(ctx)
				scopedDataDeps, err := di.Get(ctx,
					"namespacelabs.dev/foundation/std/testdata/scopes", "ScopedDataDeps")
				if err != nil {
					return nil, err
				}
				if deps.Two, err = scopes.ProvideScopedData(ctx, nil, scopedDataDeps.(*scopes.ScopedDataDeps)); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", "ExtensionDeps")
			if err != nil {
				return err
			}
			return metrics.Prepare(ctx, extensionDeps.(*metrics.ExtensionDeps))
		},
	})

	di.AddInitializer(core.Initializer{
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
