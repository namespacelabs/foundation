// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
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
	di := fninit.MakeInitializer()

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &metrics.SingletonDeps{}
			var err error
			var caller fninit.Caller
			{
				caller = cf.MakeCaller("Interceptors")
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &tracing.SingletonDeps{}
			var err error
			var caller fninit.Caller
			{
				caller = cf.MakeCaller("Interceptors")
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &creds.SingletonDeps{}
			var err error
			var caller fninit.Caller
			{
				// name: "postgres-password-file"
				p := &secrets.Secret{}
				fninit.MustUnwrapProto("ChZwb3N0Z3Jlcy1wYXNzd29yZC1maWxl", p)

				caller = cf.MakeCaller("Password")
				if deps.Password, err = secrets.ProvideSecret(ctx, caller, p); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &incluster.SingletonDeps{}
			var err error
			var caller fninit.Caller
			{
				caller = cf.MakeCaller("Creds")
				singletonDeps, err := di.GetSingleton(ctx, caller, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = creds.ProvideCreds(ctx, caller, nil, singletonDeps.(*creds.SingletonDeps)); err != nil {
					return nil, err
				}
			}

			{
				caller = cf.MakeCaller("ReadinessCheck")
				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/list",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &list.ServiceDeps{}
			var err error
			var caller fninit.Caller
			{
				// name: "list"
				p := &incluster.Database{}
				fninit.MustUnwrapProto("CgRsaXN0", p)

				caller = cf.MakeCaller("Db")
				singletonDeps, err := di.GetSingleton(ctx, caller, "namespacelabs.dev/foundation/universe/db/postgres/incluster", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Db, err = incluster.ProvideDatabase(ctx, caller, p, singletonDeps.(*incluster.SingletonDeps)); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.AddInitializer(fninit.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", "SingletonDeps")
			if err != nil {
				return err
			}
			return metrics.Prepare(ctx, singletonDeps.(*metrics.SingletonDeps))
		},
	})

	di.AddInitializer(fninit.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", "SingletonDeps")
			if err != nil {
				return err
			}
			return tracing.Prepare(ctx, singletonDeps.(*tracing.SingletonDeps))
		},
	})

	server = &ServerDeps{}

	listDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/testdata/service/list", "ServiceDeps")
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
