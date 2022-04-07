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
	"namespacelabs.dev/foundation/std/testdata/datastore"
	"namespacelabs.dev/foundation/std/testdata/service/post"
	"namespacelabs.dev/foundation/std/testdata/service/simple"
)

type ServerDeps struct {
	post post.ServiceDeps
}

func PrepareDeps(ctx context.Context) (*ServerDeps, error) {
	var server ServerDeps
	var di core.DepInitializer
	var metrics0 metrics.SingletonDeps

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

	var tracing0 tracing.SingletonDeps

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

	var datastore0 datastore.SingletonDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/core",
		Instance:    "datastore0",
		Do: func(ctx context.Context) (err error) {
			if datastore0.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", nil); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Instance:    "server.post",
		DependsOn:   []string{"deadlines0"}, Do: func(ctx context.Context) (err error) {
			// configuration: {
			//   service_name: "PostService"
			//   method_name: "*"
			//   maximum_deadline: 5
			// }
			p := &deadlines.Deadline{}
			core.MustUnwrapProto("ChUKC1Bvc3RTZXJ2aWNlEgEqHQAAoEA=", p)

			if server.post.Dl, err = deadlines.ProvideDeadlines(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p, deadlines0); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/testdata/datastore",
		Instance:    "server.post",
		DependsOn:   []string{"datastore0"}, Do: func(ctx context.Context) (err error) {
			// name: "main"
			// schema_file: {
			//   path: "schema.txt"
			//   contents: "just a test file"
			// }
			p := &datastore.Database{}
			core.MustUnwrapProto("CgRtYWluEh4KCnNjaGVtYS50eHQSEGp1c3QgYSB0ZXN0IGZpbGU=", p)

			var deps DatabaseDeps

			if deps.Cert, err = Foo(); err != nil {
				return err
			}

			if deps.Gen, err = Foo(); err != nil {
				return err
			}

			if deps.Keygen, err = Foo(); err != nil {
				return err
			}

			if server.post.Main, err = datastore.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p, datastore0, deps); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc",
		Instance:    "server.post",
		Do: func(ctx context.Context) (err error) {
			// package_name: "namespacelabs.dev/foundation/std/testdata/service/simple"
			p := &grpc.Backend{}
			core.MustUnwrapProto("CjhuYW1lc3BhY2VsYWJzLmRldi9mb3VuZGF0aW9uL3N0ZC90ZXN0ZGF0YS9zZXJ2aWNlL3NpbXBsZQ==", p)

			if server.post.SimpleConn, err = grpc.ProvideConn(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p); err != nil {
				return err
			}

			server.post.Simple = simple.NewEmptyServiceClient(server.post.SimpleConn)
			return nil
		},
	})

	var logging0 logging.ExtensionDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/interceptors",
		Instance:    "logging0",
		Do: func(ctx context.Context) (err error) {
			if logging0.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, "namespacelabs.dev/foundation/std/grpc/logging", nil); err != nil {
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

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		DependsOn:   []string{"deadlines0"},
		Do: func(ctx context.Context) error {
			return deadlines.Prepare(ctx, deadlines0)
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		DependsOn:   []string{"logging0"},
		Do: func(ctx context.Context) error {
			return logging.Prepare(ctx, logging0)
		},
	})

	return &server, di.Wait(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	post.WireService(ctx, srv, server.post)
	srv.RegisterGrpcGateway(post.RegisterPostServiceHandler)
}
