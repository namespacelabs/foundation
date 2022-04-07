// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/testdata/service/list"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"
)

type ServerDeps struct {
	list list.ServiceDeps
}

func PrepareDeps(ctx context.Context) (server *ServerDeps, err error) {
	var di core.DepInitializer

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Instance:    "metricsSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps metrics.SingletonDeps
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
			var deps tracing.SingletonDeps
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
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Instance:    "credsSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps creds.SingletonDeps
			var err error
			{
				// name: "postgres-password-file"
				p := &secrets.Secret{}
				core.MustUnwrapProto("ChZwb3N0Z3Jlcy1wYXNzd29yZC1maWxl", p)

				if deps.Password, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", p); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster",
		Instance:    "inclusterSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps incluster.SingletonDeps
			var err error
			{

				credsSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "credsSingle")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = creds.ProvideCreds(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", nil, credsSingle.(creds.SingletonDeps)); err != nil {
					return nil, err
				}
			}

			{
				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/list",
		Instance:    "server.list",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps list.ServiceDeps
			var err error
			{
				// name: "list"
				p := &incluster.Database{}
				core.MustUnwrapProto("CgRsaXN0", p)

				inclusterSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", "inclusterSingle")
				if err != nil {
					return nil, err
				}
				if deps.Db, err = incluster.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/list", p, inclusterSingle.(incluster.SingletonDeps)); err != nil {
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
			return metrics.Prepare(ctx, metricsSingle)
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			tracingSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", "tracingSingle")
			if err != nil {
				return err
			}
			return tracing.Prepare(ctx, tracingSingle)
		},
	})

	server.list, err = di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/service/list", "server.list")
	if err != nil {
		return nil, err
	}

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	list.WireService(ctx, srv, server.list)
	srv.RegisterGrpcGateway(list.RegisterListServiceHandler)
}
