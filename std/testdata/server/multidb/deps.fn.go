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
	"namespacelabs.dev/foundation/std/testdata/service/multidb"
	"namespacelabs.dev/foundation/universe/db/maria/incluster"
	"namespacelabs.dev/foundation/universe/db/maria/incluster/creds"
	fnincluster "namespacelabs.dev/foundation/universe/db/postgres/incluster"
	fncreds "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"
)

type ServerDeps struct {
	multidb *multidb.ServiceDeps
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
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/incluster/creds",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &creds.ExtensionDeps{}
			var err error
			{
				// name: "mariadb-password-file"
				p := &secrets.Secret{}
				core.MustUnwrapProto("ChVtYXJpYWRiLXBhc3N3b3JkLWZpbGU=", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "Password").WithContext(ctx)
				if deps.Password, err = secrets.ProvideSecret(ctx, p); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/incluster",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &incluster.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Creds").WithContext(ctx)
				extensionDeps, err := di.GetSingleton(ctx,
					"namespacelabs.dev/foundation/universe/db/maria/incluster/creds", "ExtensionDeps")
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
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &fncreds.ExtensionDeps{}
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
			deps := &fnincluster.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Creds").WithContext(ctx)
				extensionDeps, err := di.GetSingleton(ctx,
					"namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = fncreds.ProvideCreds(ctx, nil, extensionDeps.(*fncreds.ExtensionDeps)); err != nil {
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
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/multidb",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &multidb.ServiceDeps{}
			var err error
			{
				// name: "mariadblist"
				// schema_file: {
				//   path: "schema_maria.sql"
				//   contents: "CREATE TABLE IF NOT EXISTS list (\n    Id INT NOT NULL AUTO_INCREMENT,\n    Item varchar(255) NOT NULL,\n    PRIMARY KEY(Id)\n);"
				// }
				p := &incluster.Database{}
				core.MustUnwrapProto("CgttYXJpYWRibGlzdBKQAQoQc2NoZW1hX21hcmlhLnNxbBJ8Q1JFQVRFIFRBQkxFIElGIE5PVCBFWElTVFMgbGlzdCAoCiAgICBJZCBJTlQgTk9UIE5VTEwgQVVUT19JTkNSRU1FTlQsCiAgICBJdGVtIHZhcmNoYXIoMjU1KSBOT1QgTlVMTCwKICAgIFBSSU1BUlkgS0VZKElkKQopOw==", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "Maria").WithContext(ctx)
				extensionDeps, err := di.GetSingleton(ctx,
					"namespacelabs.dev/foundation/universe/db/maria/incluster", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Maria, err = incluster.ProvideDatabase(ctx, p, extensionDeps.(*incluster.ExtensionDeps)); err != nil {
					return nil, err
				}
			}

			{
				// name: "postgreslist"
				p := &fnincluster.Database{}
				core.MustUnwrapProto("Cgxwb3N0Z3Jlc2xpc3Q=", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "Postgres").WithContext(ctx)
				extensionDeps, err := di.GetSingleton(ctx,
					"namespacelabs.dev/foundation/universe/db/postgres/incluster", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Postgres, err = fnincluster.ProvideDatabase(ctx, p, extensionDeps.(*fnincluster.ExtensionDeps)); err != nil {
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

	multidbDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", "ServiceDeps")
	if err != nil {
		return nil, err
	}
	server.multidb = multidbDeps.(*multidb.ServiceDeps)

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	multidb.WireService(ctx, srv, server.multidb)
	srv.RegisterGrpcGateway(multidb.RegisterListServiceHandler)
}
