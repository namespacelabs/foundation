// This file was automatically generated.
// This code uses type assertions for now. When go 1.18 is more widely deployed, it will switch to generics.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/grpc"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
	"namespacelabs.dev/foundation/std/grpc/logging"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/testdata/datastore"
	"namespacelabs.dev/foundation/std/testdata/service/post"
	"namespacelabs.dev/foundation/std/testdata/service/simple"
	"namespacelabs.dev/foundation/universe/go/panicparse"
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

	namespacelabs_dev_foundation_std_grpc_deadlines = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps deadlines.ExtensionDeps
			var err error

			if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
				return nil, err
			}

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_std_testdata_datastore = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/datastore",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps datastore.ExtensionDeps
			var err error
			{
				// name: "cert"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgRjZXJ0", p)

				if deps.Cert, err = secrets.ProvideSecret(ctx, p); err != nil {
					return nil, err
				}
			}

			{
				// name: "gen"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgNnZW4=", p)

				if deps.Gen, err = secrets.ProvideSecret(ctx, p); err != nil {
					return nil, err
				}
			}

			{
				// name: "keygen"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgZrZXlnZW4=", p)

				if deps.Keygen, err = secrets.ProvideSecret(ctx, p); err != nil {
					return nil, err
				}
			}

			{

				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, nil); err != nil {
					return nil, err
				}
			}

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_std_testdata_service_post = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/post",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps post.ServiceDeps
			var err error

			err = di.Instantiate(ctx, namespacelabs_dev_foundation_std_grpc_deadlines, func(ctx context.Context, v interface{}) (err error) {
				// configuration: {
				//   service_name: "PostService"
				//   method_name: "*"
				//   maximum_deadline: 5
				// }
				p := &deadlines.Deadline{}
				core.MustUnwrapProto("ChUKC1Bvc3RTZXJ2aWNlEgEqHQAAoEA=", p)

				if deps.Dl, err = deadlines.ProvideDeadlines(ctx, p, v.(deadlines.ExtensionDeps)); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return nil, err
			}

			err = di.Instantiate(ctx, namespacelabs_dev_foundation_std_testdata_datastore, func(ctx context.Context, v interface{}) (err error) {
				// name: "main"
				// schema_file: {
				//   path: "schema.txt"
				//   contents: "just a test file"
				// }
				p := &datastore.Database{}
				core.MustUnwrapProto("CgRtYWluEh4KCnNjaGVtYS50eHQSEGp1c3QgYSB0ZXN0IGZpbGU=", p)

				if deps.Main, err = datastore.ProvideDatabase(ctx, p, v.(datastore.ExtensionDeps)); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				return nil, err
			}

			{
				// package_name: "namespacelabs.dev/foundation/std/testdata/service/simple"
				p := &grpc.Backend{}
				core.MustUnwrapProto("CjhuYW1lc3BhY2VsYWJzLmRldi9mb3VuZGF0aW9uL3N0ZC90ZXN0ZGF0YS9zZXJ2aWNlL3NpbXBsZQ==", p)

				if deps.SimpleConn, err = grpc.ProvideConn(ctx, p); err != nil {
					return nil, err
				}

				deps.Simple = simple.NewEmptyServiceClient(deps.SimpleConn)

			}

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_std_grpc_logging = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps logging.ExtensionDeps
			var err error

			if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
				return nil, err
			}

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_universe_go_panicparse = core.Provider{
		PackageName: "namespacelabs.dev/foundation/universe/go/panicparse",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps panicparse.ExtensionDeps
			var err error

			if deps.DebugHandler, err = core.ProvideDebugHandler(ctx, nil); err != nil {
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

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, namespacelabs_dev_foundation_std_grpc_deadlines, func(ctx context.Context, v interface{}) (err error) {
				return deadlines.Prepare(ctx, v.(deadlines.ExtensionDeps))
			})
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, namespacelabs_dev_foundation_std_grpc_logging, func(ctx context.Context, v interface{}) (err error) {
				return logging.Prepare(ctx, v.(logging.ExtensionDeps))
			})
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/universe/go/panicparse",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, namespacelabs_dev_foundation_universe_go_panicparse, func(ctx context.Context, v interface{}) (err error) {
				return panicparse.Prepare(ctx, v.(panicparse.ExtensionDeps))
			})
		},
	})

}

func WireServices(ctx context.Context, srv server.Server, depgraph core.Dependencies) []error {
	var errs []error

	if err := depgraph.Instantiate(ctx, namespacelabs_dev_foundation_std_testdata_service_post, func(ctx context.Context, v interface{}) error {
		post.WireService(ctx, srv.Scope(namespacelabs_dev_foundation_std_testdata_service_post.PackageName), v.(post.ServiceDeps))
		return nil
	}); err != nil {
		errs = append(errs, err)
	}

	srv.InternalRegisterGrpcGateway(post.RegisterPostServiceHandler)

	return errs
}
