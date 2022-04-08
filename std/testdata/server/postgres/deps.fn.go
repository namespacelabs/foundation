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
	list *list.ServiceDeps
}

func PrepareDeps(ctx context.Context) (server *ServerDeps, err error) {
	di := core.MakeInitializer()

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Typename:    "SingletonDeps",
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
		Typename:    "SingletonDeps",
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
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Typename:    "SingletonDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &creds.SingletonDeps{}
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
		Typename:    "SingletonDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &incluster.SingletonDeps{}
			var err error
			{

				singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = creds.ProvideCreds(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", nil, singletonDeps.(*creds.SingletonDeps)); err != nil {
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
		Typename:    "ServiceDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &list.ServiceDeps{}
			var err error
			{
				// name: "list"
				p := &incluster.Database{}
				core.MustUnwrapProto("CgRsaXN0", p)

				singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Db, err = incluster.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/list", p, singletonDeps.(*incluster.SingletonDeps)); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", "SingletonDeps")
			if err != nil {
				return err
			}
			return metrics.Prepare(ctx, singletonDeps.(*metrics.SingletonDeps))
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", "SingletonDeps")
			if err != nil {
				return err
			}
			return tracing.Prepare(ctx, singletonDeps.(*tracing.SingletonDeps))
		},
	})

	server = &ServerDeps{}

	listDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/service/list", "ServiceDeps")
	if err != nil {
		return nil, err
	}
	server.list = listDeps.(*list.ServiceDeps)

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	list.WireService(ctx, srv, server.list)
	srv.RegisterGrpcGateway(list.RegisterListServiceHandler)
}
