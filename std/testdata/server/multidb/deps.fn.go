// This file was automatically generated.
package main

import (
	"context"
	"fmt"

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
	multidb multidb.ServiceDeps
}

func PrepareDeps(ctx context.Context) (server *ServerDeps, err error) {
	di := core.MakeInitializer()

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Instance:    "metricsSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *metrics.SingletonDeps
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
			var deps *tracing.SingletonDeps
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
		Instance:    "credsSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *creds.SingletonDeps
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
		Instance:    "inclusterSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *incluster.SingletonDeps
			var err error
			{

				credsSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster/creds", "credsSingle")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = creds.ProvideCreds(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster", nil, credsSingle.(creds.SingletonDeps)); err != nil {
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
		Instance:    "credsSingle1",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *fncreds.SingletonDeps
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
		Instance:    "inclusterSingle1",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *fnincluster.SingletonDeps
			var err error
			{

				credsSingle1, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", "credsSingle1")
				if err != nil {
					return nil, err
				}
				if deps.Creds, err = fncreds.ProvideCreds(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", nil, credsSingle1.(fncreds.SingletonDeps)); err != nil {
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
		Instance:    "multidbDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *multidb.ServiceDeps
			var err error
			{
				// name: "mariadblist"
				// schema_file: {
				//   path: "schema_maria.sql"
				//   contents: "CREATE TABLE IF NOT EXISTS list (\n    Id INT NOT NULL AUTO_INCREMENT,\n    Item varchar(255) NOT NULL,\n    PRIMARY KEY(Id)\n);"
				// }
				p := &incluster.Database{}
				core.MustUnwrapProto("CgttYXJpYWRibGlzdBKQAQoQc2NoZW1hX21hcmlhLnNxbBJ8Q1JFQVRFIFRBQkxFIElGIE5PVCBFWElTVFMgbGlzdCAoCiAgICBJZCBJTlQgTk9UIE5VTEwgQVVUT19JTkNSRU1FTlQsCiAgICBJdGVtIHZhcmNoYXIoMjU1KSBOT1QgTlVMTCwKICAgIFBSSU1BUlkgS0VZKElkKQopOw==", p)

				inclusterSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster", "inclusterSingle")
				if err != nil {
					return nil, err
				}
				if deps.Maria, err = incluster.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", p, inclusterSingle.(incluster.SingletonDeps)); err != nil {
					return nil, err
				}
			}

			{
				// name: "postgreslist"
				p := &fnincluster.Database{}
				core.MustUnwrapProto("Cgxwb3N0Z3Jlc2xpc3Q=", p)

				inclusterSingle1, err := di.Get(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", "inclusterSingle1")
				if err != nil {
					return nil, err
				}
				if deps.Postgres, err = fnincluster.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", p, inclusterSingle1.(fnincluster.SingletonDeps)); err != nil {
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
			return metrics.Prepare(ctx, metricsSingle.(metrics.SingletonDeps))
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			tracingSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", "tracingSingle")
			if err != nil {
				return err
			}
			return tracing.Prepare(ctx, tracingSingle.(tracing.SingletonDeps))
		},
	})

	var ok bool

	multidbDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", "multidbDeps")
	if err != nil {
		return nil, err
	}
	if server.multidb, ok = multidbDeps.(multidb.ServiceDeps); !ok {
		return nil, fmt.Errorf("multidbDeps is not of type multidb.ServiceDeps")
	}

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	multidb.WireService(ctx, srv, server.multidb)
	srv.RegisterGrpcGateway(multidb.RegisterListServiceHandler)
}
