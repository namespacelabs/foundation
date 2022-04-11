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

	di.Add(fninit.Provider{
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

	di.Add(fninit.Provider{
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

	di.Add(fninit.Provider{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &creds.ExtensionDeps{}
			var err error
			{
				// name: "postgres-password-file"
				p := &secrets.Secret{}
				fninit.MustUnwrapProto("ChZwb3N0Z3Jlcy1wYXNzd29yZC1maWxl", p)

				caller := cf.ForInstance("Password")
				if deps.Password, err = secrets.ProvideSecret(ctx, caller, p); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Provider{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &incluster.ExtensionDeps{}
			var err error
			{
				caller := cf.ForInstance("Creds")
				extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = creds.ProvideCreds(ctx, caller, nil, extensionDeps.(*creds.ExtensionDeps)); err != nil {
					return nil, err
				}
			}

			{
				caller := cf.ForInstance("ReadinessCheck")
				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/list",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &list.ServiceDeps{}
			var err error
			{
				// name: "list"
				p := &incluster.Database{}
				fninit.MustUnwrapProto("CgRsaXN0", p)

				caller := cf.ForInstance("Db")
				extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Db, err = incluster.ProvideDatabase(ctx, caller, p, extensionDeps.(*incluster.ExtensionDeps)); err != nil {
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
