// This file was automatically generated.
package main

import (
	"context"
	"fmt"

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
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Instance:    "deadlinesSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *deadlines.SingletonDeps
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
		Instance:    "datastoreSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *datastore.SingletonDeps
			var err error
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
		PackageName: "namespacelabs.dev/foundation/std/testdata/datastore",
		Instance:    "datastore0",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *datastore.DatabaseDeps
			var err error
			{
				// name: "cert"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgRjZXJ0", p)

				if deps.Cert, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", p); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/post",
		Instance:    "postDeps",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *post.ServiceDeps
			var err error
			{
				// configuration: {
				//   service_name: "PostService"
				//   method_name: "*"
				//   maximum_deadline: 5
				// }
				p := &deadlines.Deadline{}
				core.MustUnwrapProto("ChUKC1Bvc3RTZXJ2aWNlEgEqHQAAoEA=", p)

				deadlinesSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/std/grpc/deadlines", "deadlinesSingle")
				if err != nil {
					return nil, err
				}
				if deps.Dl, err = deadlines.ProvideDeadlines(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p, deadlinesSingle.(deadlines.SingletonDeps)); err != nil {
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

				datastoreSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", "datastoreSingle")
				if err != nil {
					return nil, err
				}

				datastore0, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", "datastore0")
				if err != nil {
					return nil, err
				}
				if deps.Main, err = datastore.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p, datastoreSingle.(datastore.SingletonDeps), datastore0.(datastore.DatabaseDeps)); err != nil {
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

				deps.Simple = simple.NewEmptyServiceClient(postDeps.SimpleConn)
			}
			return deps, err
		},
	})

	di.Add(core.Factory{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Instance:    "loggingSingle",
		Singleton:   true,
		Do: func(ctx context.Context) (interface{}, error) {
			var deps *logging.SingletonDeps
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

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Do: func(ctx context.Context) error {
			deadlinesSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/std/grpc/deadlines", "deadlinesSingle")
			if err != nil {
				return err
			}
			return deadlines.Prepare(ctx, deadlinesSingle.(deadlines.SingletonDeps))
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Do: func(ctx context.Context) error {
			loggingSingle, err := di.Get(ctx, "namespacelabs.dev/foundation/std/grpc/logging", "loggingSingle")
			if err != nil {
				return err
			}
			return logging.Prepare(ctx, loggingSingle.(logging.SingletonDeps))
		},
	})

	var ok bool

	postDeps, err := di.Get(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", "postDeps")
	if err != nil {
		return nil, err
	}
	if server.post, ok = postDeps.(post.ServiceDeps); !ok {
		return nil, fmt.Errorf("postDeps is not of type post.ServiceDeps")
	}

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	post.WireService(ctx, srv, server.post)
	srv.RegisterGrpcGateway(post.RegisterPostServiceHandler)
}
