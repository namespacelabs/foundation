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
	multidb multidb.ServiceDeps
}

// This code uses type assertions for now. When go 1.18 is more widely deployed, it will switch to generics.
func PrepareDeps(ctx context.Context) (server *ServerDeps, err error) {
	di := core.MakeInitializer()

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps metrics.ExtensionDeps
			var err error

			if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
				return nil, err
			}

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps tracing.ExtensionDeps
			var err error

			if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
				return nil, err
			}

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/universe/db/maria/incluster/creds",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps creds.ExtensionDeps
			var err error
			// name: "mariadb-password-file"
			p := &secrets.Secret{}
			core.MustUnwrapProto("ChVtYXJpYWRiLXBhc3N3b3JkLWZpbGU=", p)

			if deps.Password, err = secrets.ProvideSecret(ctx, p); err != nil {
				return nil, err
			}

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/universe/db/maria/incluster",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps incluster.ExtensionDeps
			var err error

			err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/universe/db/maria/incluster/creds"},
				func(ctx context.Context, v interface{}) (err error) {

					if deps.Creds, err = creds.ProvideCreds(ctx, nil, v.(creds.ExtensionDeps)); err != nil {
						return err
					}
					return nil
				})
			if err != nil {
				return nil, err
			}

			{

				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, nil); err != nil {
					return nil, err
				}
			}

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps fncreds.ExtensionDeps
			var err error
			// name: "postgres-password-file"
			p := &secrets.Secret{}
			core.MustUnwrapProto("ChZwb3N0Z3Jlcy1wYXNzd29yZC1maWxl", p)

			if deps.Password, err = secrets.ProvideSecret(ctx, p); err != nil {
				return nil, err
			}

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/universe/db/postgres/incluster",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps fnincluster.ExtensionDeps
			var err error

			err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"},
				func(ctx context.Context, v interface{}) (err error) {

					if deps.Creds, err = fncreds.ProvideCreds(ctx, nil, v.(fncreds.ExtensionDeps)); err != nil {
						return err
					}
					return nil
				})
			if err != nil {
				return nil, err
			}

			{

				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, nil); err != nil {
					return nil, err
				}
			}

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/std/testdata/service/multidb",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps multidb.ServiceDeps
			var err error

			err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/universe/db/maria/incluster"},
				func(ctx context.Context, v interface{}) (err error) {
					// name: "mariadblist"
					// schema_file: {
					//   path: "schema_maria.sql"
					//   contents: "CREATE TABLE IF NOT EXISTS list (\n    Id INT NOT NULL AUTO_INCREMENT,\n    Item varchar(255) NOT NULL,\n    PRIMARY KEY(Id)\n);"
					// }
					p := &incluster.Database{}
					core.MustUnwrapProto("CgttYXJpYWRibGlzdBKQAQoQc2NoZW1hX21hcmlhLnNxbBJ8Q1JFQVRFIFRBQkxFIElGIE5PVCBFWElTVFMgbGlzdCAoCiAgICBJZCBJTlQgTk9UIE5VTEwgQVVUT19JTkNSRU1FTlQsCiAgICBJdGVtIHZhcmNoYXIoMjU1KSBOT1QgTlVMTCwKICAgIFBSSU1BUlkgS0VZKElkKQopOw==", p)

					if deps.Maria, err = incluster.ProvideDatabase(ctx, p, v.(incluster.ExtensionDeps)); err != nil {
						return err
					}
					return nil
				})
			if err != nil {
				return nil, err
			}

			err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/universe/db/postgres/incluster"},
				func(ctx context.Context, v interface{}) (err error) {
					// name: "postgreslist"
					p := &fnincluster.Database{}
					core.MustUnwrapProto("Cgxwb3N0Z3Jlc2xpc3Q=", p)

					if deps.Postgres, err = fnincluster.ProvideDatabase(ctx, p, v.(fnincluster.ExtensionDeps)); err != nil {
						return err
					}
					return nil
				})
			if err != nil {
				return nil, err
			}

			return deps, err
		},
	})

	server = &ServerDeps{}
	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/testdata/server/multidb",
		Do: func(ctx context.Context) error {

			err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/testdata/service/multidb"},
				func(ctx context.Context, v interface{}) (err error) {
					server.multidb = v.(multidb.ServiceDeps)
					return nil
				})
			if err != nil {
				return err
			}

			return nil
		},
	})
	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/go/grpc/metrics"},
				func(ctx context.Context, v interface{}) (err error) {
					return metrics.Prepare(ctx, v.(metrics.ExtensionDeps))
				})
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/monitoring/tracing"},
				func(ctx context.Context, v interface{}) (err error) {
					return tracing.Prepare(ctx, v.(tracing.ExtensionDeps))
				})
		},
	})

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	multidb.WireService(ctx, srv, server.multidb)
	srv.RegisterGrpcGateway(multidb.RegisterListServiceHandler)
}
