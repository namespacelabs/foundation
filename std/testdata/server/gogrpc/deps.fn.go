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
	post *post.ServiceDeps
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
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Typename:    "SingletonDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &deadlines.SingletonDeps{}
			var err error
			{
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, "namespacelabs.dev/foundation/std/grpc/deadlines", nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/datastore",
		Typename:    "SingletonDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &datastore.SingletonDeps{}
			var err error
			{
				// name: "cert"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgRjZXJ0", p)

				if deps.Cert, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", p); err != nil {
					return nil, err
				}
			}

			{
				// name: "gen"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgNnZW4=", p)

				if deps.Gen, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", p); err != nil {
					return nil, err
				}
			}

			{
				// name: "keygen"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgZrZXlnZW4=", p)

				if deps.Keygen, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", p); err != nil {
					return nil, err
				}
			}

			{
				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/post",
		Typename:    "ServiceDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &post.ServiceDeps{}
			var err error
			{
				// configuration: {
				//   service_name: "PostService"
				//   method_name: "*"
				//   maximum_deadline: 5
				// }
				p := &deadlines.Deadline{}
				core.MustUnwrapProto("ChUKC1Bvc3RTZXJ2aWNlEgEqHQAAoEA=", p)

				singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/grpc/deadlines", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Dl, err = deadlines.ProvideDeadlines(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p, singletonDeps.(*deadlines.SingletonDeps)); err != nil {
					return nil, err
				}
			}

			{
				// name: "main"
				// schema_file: {
				//   path: "schema.txt"
				//   contents: "just a test file"
				// }
				p := &datastore.Database{}
				core.MustUnwrapProto("CgRtYWluEh4KCnNjaGVtYS50eHQSEGp1c3QgYSB0ZXN0IGZpbGU=", p)

				singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Main, err = datastore.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p, singletonDeps.(*datastore.SingletonDeps)); err != nil {
					return nil, err
				}
			}

			{
				// package_name: "namespacelabs.dev/foundation/std/testdata/service/simple"
				p := &grpc.Backend{}
				core.MustUnwrapProto("CjhuYW1lc3BhY2VsYWJzLmRldi9mb3VuZGF0aW9uL3N0ZC90ZXN0ZGF0YS9zZXJ2aWNlL3NpbXBsZQ==", p)

				if deps.SimpleConn, err = grpc.ProvideConn(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p); err != nil {
					return nil, err
				}

				deps.Simple = simple.NewEmptyServiceClient(deps.SimpleConn)
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Typename:    "SingletonDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			deps := &logging.SingletonDeps{}
			var err error
			{
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, "namespacelabs.dev/foundation/std/grpc/logging", nil); err != nil {
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

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/grpc/deadlines", "SingletonDeps")
			if err != nil {
				return err
			}
			return deadlines.Prepare(ctx, singletonDeps.(*deadlines.SingletonDeps))
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/grpc/logging", "SingletonDeps")
			if err != nil {
				return err
			}
			return logging.Prepare(ctx, singletonDeps.(*logging.SingletonDeps))
		},
	})

	server = &ServerDeps{}

	postDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", "ServiceDeps")
	if err != nil {
		return nil, err
	}
	server.post = postDeps.(*post.ServiceDeps)

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	post.WireService(ctx, srv, server.post)
	srv.RegisterGrpcGateway(post.RegisterPostServiceHandler)
}
