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
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/incluster/creds",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &creds.ExtensionDeps{}
			var err error
			{
				// name: "mariadb-password-file"
				p := &secrets.Secret{}
				fninit.MustUnwrapProto("ChVtYXJpYWRiLXBhc3N3b3JkLWZpbGU=", p)

				caller := cf.ForInstance("Password")
				if deps.Password, err = secrets.ProvideSecret(ctx, caller, p); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/incluster",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &incluster.ExtensionDeps{}
			var err error
			{
				caller := cf.ForInstance("Creds")
				extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster/creds", "ExtensionDeps")
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

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &fncreds.ExtensionDeps{}
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

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &fnincluster.ExtensionDeps{}
			var err error
			{
				caller := cf.ForInstance("Creds")
				extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = fncreds.ProvideCreds(ctx, caller, nil, extensionDeps.(*fncreds.ExtensionDeps)); err != nil {
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

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/multidb",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &multidb.ServiceDeps{}
			var err error
			{
				// name: "mariadblist"
				// schema_file: {
				//   path: "schema_maria.sql"
				//   contents: "CREATE TABLE IF NOT EXISTS list (\n    Id INT NOT NULL AUTO_INCREMENT,\n    Item varchar(255) NOT NULL,\n    PRIMARY KEY(Id)\n);"
				// }
				p := &incluster.Database{}
				fninit.MustUnwrapProto("CgttYXJpYWRibGlzdBKQAQoQc2NoZW1hX21hcmlhLnNxbBJ8Q1JFQVRFIFRBQkxFIElGIE5PVCBFWElTVFMgbGlzdCAoCiAgICBJZCBJTlQgTk9UIE5VTEwgQVVUT19JTkNSRU1FTlQsCiAgICBJdGVtIHZhcmNoYXIoMjU1KSBOT1QgTlVMTCwKICAgIFBSSU1BUlkgS0VZKElkKQopOw==", p)

				caller := cf.ForInstance("Maria")
				extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Maria, err = incluster.ProvideDatabase(ctx, caller, p, extensionDeps.(*incluster.ExtensionDeps)); err != nil {
					return nil, err
				}
			}

			{
				// name: "postgreslist"
				p := &fnincluster.Database{}
				fninit.MustUnwrapProto("Cgxwb3N0Z3Jlc2xpc3Q=", p)

				caller := cf.ForInstance("Postgres")
				extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Postgres, err = fnincluster.ProvideDatabase(ctx, caller, p, extensionDeps.(*fnincluster.ExtensionDeps)); err != nil {
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
