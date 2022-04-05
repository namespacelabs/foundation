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
	"namespacelabs.dev/foundation/universe/db/maria/creds"
	"namespacelabs.dev/foundation/universe/db/maria/incluster"
	fnincluster "namespacelabs.dev/foundation/universe/db/postgres/incluster"
	fncreds "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"
)

type ServerDeps struct {
	multidb multidb.ServiceDeps
}

func PrepareDeps(ctx context.Context) (*ServerDeps, error) {
	var server ServerDeps
	var di core.DepInitializer
	var metrics0 metrics.ExtensionDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/interceptors",
		Instance:    "metrics0",
		Do: func(ctx context.Context) (err error) {
			if metrics0.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", nil); err != nil {
				return err
			}
			return nil
		},
	})

	var tracing0 tracing.ExtensionDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/interceptors",
		Instance:    "tracing0",
		Do: func(ctx context.Context) (err error) {
			if tracing0.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", nil); err != nil {
				return err
			}
			return nil
		},
	})

	var creds0 creds.ExtensionDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/secrets",
		Instance:    "creds0",
		Do: func(ctx context.Context) (err error) {
			// name: "mariadb-password-file"
			p := &secrets.Secret{}
			core.MustUnwrapProto("ChVtYXJpYWRiLXBhc3N3b3JkLWZpbGU=", p)

			if creds0.Password, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/universe/db/maria/creds", p); err != nil {
				return err
			}
			return nil
		},
	})

	var incluster0 incluster.ExtensionDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/creds",
		Instance:    "incluster0",
		DependsOn:   []string{"creds0"}, Do: func(ctx context.Context) (err error) {
			if incluster0.Creds, err = creds.ProvideCreds(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster", nil, creds0); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/core",
		Instance:    "incluster0",
		Do: func(ctx context.Context) (err error) {
			if incluster0.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, "namespacelabs.dev/foundation/universe/db/maria/incluster", nil); err != nil {
				return err
			}
			return nil
		},
	})

	var creds2 fncreds.ExtensionDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/secrets",
		Instance:    "creds2",
		Do: func(ctx context.Context) (err error) {
			// name: "postgres-password-file"
			p := &secrets.Secret{}
			core.MustUnwrapProto("ChZwb3N0Z3Jlcy1wYXNzd29yZC1maWxl", p)

			if creds2.Password, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds", p); err != nil {
				return err
			}
			return nil
		},
	})

	var incluster2 fnincluster.ExtensionDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Instance:    "incluster2",
		DependsOn:   []string{"creds2"}, Do: func(ctx context.Context) (err error) {
			if incluster2.Creds, err = fncreds.ProvideCreds(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", nil, creds2); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/core",
		Instance:    "incluster2",
		Do: func(ctx context.Context) (err error) {
			if incluster2.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, "namespacelabs.dev/foundation/universe/db/postgres/incluster", nil); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/incluster",
		Instance:    "server.multidb",
		DependsOn:   []string{"incluster0"}, Do: func(ctx context.Context) (err error) {
			// name: "mariadblist"
			// schema_file: {
			//   path: "schema_maria.sql"
			//   contents: "CREATE TABLE IF NOT EXISTS list (\n    Id INT NOT NULL AUTO_INCREMENT,\n    Item varchar(255) NOT NULL,\n    PRIMARY KEY(Id)\n);"
			// }
			p := &incluster.Database{}
			core.MustUnwrapProto("CgttYXJpYWRibGlzdBKQAQoQc2NoZW1hX21hcmlhLnNxbBJ8Q1JFQVRFIFRBQkxFIElGIE5PVCBFWElTVFMgbGlzdCAoCiAgICBJZCBJTlQgTk9UIE5VTEwgQVVUT19JTkNSRU1FTlQsCiAgICBJdGVtIHZhcmNoYXIoMjU1KSBOT1QgTlVMTCwKICAgIFBSSU1BUlkgS0VZKElkKQopOw==", p)

			if server.multidb.Maria, err = incluster.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", p, incluster0); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster",
		Instance:    "server.multidb",
		DependsOn:   []string{"incluster2"}, Do: func(ctx context.Context) (err error) {
			// name: "postgreslist"
			p := &fnincluster.Database{}
			core.MustUnwrapProto("Cgxwb3N0Z3Jlc2xpc3Q=", p)

			if server.multidb.Postgres, err = fnincluster.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/multidb", p, incluster2); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		DependsOn:   []string{"metrics0"},
		Do: func(ctx context.Context) error {
			return metrics.Prepare(ctx, metrics0)
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		DependsOn:   []string{"tracing0"},
		Do: func(ctx context.Context) error {
			return tracing.Prepare(ctx, tracing0)
		},
	})

	return &server, di.Wait(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	multidb.WireService(ctx, srv, server.multidb)
	srv.RegisterGrpcGateway(multidb.RegisterListServiceHandler)
}
