// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/grpc"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
	"namespacelabs.dev/foundation/std/grpc/logging"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/testdata/datastore"
	"namespacelabs.dev/foundation/std/testdata/service/post"
	"namespacelabs.dev/foundation/std/testdata/service/simple"
)

type ServerDeps struct {
	post post.ServiceDeps
}

// This code uses type assertions for now. When go 1.18 is more common, it will switch to generics.
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
		Package: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps deadlines.ExtensionDeps
			var err error

			if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
				return nil, err
			}

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/std/testdata/datastore",
		Do: func(ctx context.Context) (interface{}, error) {
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

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/std/testdata/service/post",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps post.ServiceDeps
			var err error

			err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/grpc/deadlines"},
				func(ctx context.Context, v interface{}) (err error) {
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

			err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/testdata/datastore"},
				func(ctx context.Context, v interface{}) (err error) {
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

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/std/grpc/logging",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps logging.ExtensionDeps
			var err error

			if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
				return nil, err
			}

			return deps, err
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

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/grpc/deadlines"},
				func(ctx context.Context, v interface{}) (err error) {
					return deadlines.Prepare(ctx, v.(deadlines.ExtensionDeps))
				})
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Do: func(ctx context.Context) error {
			return di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/grpc/logging"},
				func(ctx context.Context, v interface{}) (err error) {
					return logging.Prepare(ctx, v.(logging.ExtensionDeps))
				})
		},
	})

	server = &ServerDeps{}

	err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/testdata/service/post"},
		func(ctx context.Context, v interface{}) (err error) {
			server.post = v.(post.ServiceDeps)
			return nil
		})
	if err != nil {
		return nil, err
	}

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	post.WireService(ctx, srv, server.post)
	srv.RegisterGrpcGateway(post.RegisterPostServiceHandler)
}
