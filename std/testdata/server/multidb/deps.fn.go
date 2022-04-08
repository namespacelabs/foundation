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
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/incluster/creds",
		Typename:    "SingletonDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &creds.SingletonDeps{}
			var err error
			{
				// name: "mariadb-password-file"
				p := &secrets.Secret{}
				core.MustUnwrapProto("ChVtYXJpYWRiLXBhc3N3b3JkLWZpbGU=", p)

				if deps.Password, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster/creds", p); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/incluster",
		Typename:    "SingletonDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &incluster.SingletonDeps{}
			var err error
			{

				singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster/creds", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = creds.ProvideCreds(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster", nil, singletonDeps.(*creds.SingletonDeps)); err != nil {
					return nil, err
				}
			}

			{
				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster", nil); err != nil {
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
			deps := &fncreds.SingletonDeps{}
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
			deps := &fnincluster.SingletonDeps{}
			var err error
			{

				singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = fncreds.ProvideCreds(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", nil, singletonDeps.(*fncreds.SingletonDeps)); err != nil {
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
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/multidb",
		Typename:    "ServiceDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
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

				singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Maria, err = incluster.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", p, singletonDeps.(*incluster.SingletonDeps)); err != nil {
					return nil, err
				}
			}

			{
				// name: "postgreslist"
				p := &fnincluster.Database{}
				core.MustUnwrapProto("Cgxwb3N0Z3Jlc2xpc3Q=", p)

				singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Postgres, err = fnincluster.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", p, singletonDeps.(*fnincluster.SingletonDeps)); err != nil {
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

	multidbDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", "ServiceDeps")
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
