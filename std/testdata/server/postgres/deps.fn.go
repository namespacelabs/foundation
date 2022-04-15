// This file was automatically generated.
// This code uses type assertions for now. When go 1.18 is more widely deployed, it will switch to generics.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/testdata/service/list"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster/creds"
)

var (
	namespacelabs_dev_foundation_std_go_grpc_metrics = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps metrics.ExtensionDeps
			var err error

			if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
				return nil, err
			}

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_std_monitoring_tracing = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps tracing.ExtensionDeps
			var err error

			if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
				return nil, err
			}

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_universe_db_postgres_incluster_creds = core.Provider{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster/creds",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps creds.ExtensionDeps
			var err error
			// name: "postgres-password-file"
			p := &secrets.Secret{}
			core.MustUnwrapProto("ChZwb3N0Z3Jlcy1wYXNzd29yZC1maWxl", p)

			if deps.Password, err = secrets.ProvideSecret(ctx, p); err != nil {
				return nil, err
			}

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_universe_db_postgres_incluster = core.Provider{
		PackageName: "namespacelabs.dev/foundation/universe/db/postgres/incluster",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps incluster.ExtensionDeps
			var err error

			err = di.Instantiate(ctx, namespacelabs_dev_foundation_universe_db_postgres_incluster_creds, func(ctx context.Context, v interface{}) (err error) {

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

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_std_testdata_service_list = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/list",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps list.ServiceDeps
			var err error

			err = di.Instantiate(ctx, namespacelabs_dev_foundation_universe_db_postgres_incluster, func(ctx context.Context, v interface{}) (err error) {
				// name: "list"
				p := &incluster.Database{}
				core.MustUnwrapProto("CgRsaXN0", p)

				if deps.Db, err = incluster.ProvideDatabase(ctx, p, v.(incluster.ExtensionDeps)); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return nil, err
			}

			return deps, nil
		},
	}
)

func RegisterInitializers(di *core.DependencyGraph) {
	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, namespacelabs_dev_foundation_std_go_grpc_metrics, func(ctx context.Context, v interface{}) (err error) {
				return metrics.Prepare(ctx, v.(metrics.ExtensionDeps))
			})
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, namespacelabs_dev_foundation_std_monitoring_tracing, func(ctx context.Context, v interface{}) (err error) {
				return tracing.Prepare(ctx, v.(tracing.ExtensionDeps))
			})
		},
	})

}

func WireServices(ctx context.Context, srv server.Server, depgraph core.Dependencies) []error {
	var errs []error

	if err := depgraph.Instantiate(ctx, namespacelabs_dev_foundation_std_testdata_service_list, func(ctx context.Context, v interface{}) error {
		list.WireService(ctx, srv.Scope(namespacelabs_dev_foundation_std_testdata_service_list.PackageName), v.(list.ServiceDeps))
		return nil
	}); err != nil {
		errs = append(errs, err)
	}

	srv.InternalRegisterGrpcGateway(list.RegisterListServiceHandler)

	return errs
}
