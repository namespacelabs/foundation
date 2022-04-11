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
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &creds.ExtensionDeps{}
			var err error
			{
				// name: "postgres-password-file"
				p := &secrets.Secret{}
				core.MustUnwrapProto("ChZwb3N0Z3Jlcy1wYXNzd29yZC1maWxl", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "Password").WithContext(ctx)
				if deps.Password, err = secrets.ProvideSecret(ctx, p); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &incluster.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Creds").WithContext(ctx)
				extensionDeps, err := di.GetSingleton(ctx,
					"namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = creds.ProvideCreds(ctx, nil, extensionDeps.(*creds.ExtensionDeps)); err != nil {
					return nil, err
				}
			}

			{
				ctx = core.PathFromContext(ctx).Append(pkg, "ReadinessCheck").WithContext(ctx)
				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/list",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &list.ServiceDeps{}
			var err error
			{
				// name: "list"
				p := &incluster.Database{}
				core.MustUnwrapProto("CgRsaXN0", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "Db").WithContext(ctx)
				extensionDeps, err := di.GetSingleton(ctx,
					"namespacelabs.dev/foundation/universe/db/postgres/incluster", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Db, err = incluster.ProvideDatabase(ctx, p, extensionDeps.(*incluster.ExtensionDeps)); err != nil {
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
