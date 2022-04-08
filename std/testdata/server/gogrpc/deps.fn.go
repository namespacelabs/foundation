// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	fninit "namespacelabs.dev/foundation/std/go/core/init"
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
	di := fninit.MakeInitializer()

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &metrics.SingletonDeps{}
			var err error
			{
				caller := cf.ForInstance("Interceptors")
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &tracing.SingletonDeps{}
			var err error
			{
				caller := cf.ForInstance("Interceptors")
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &deadlines.SingletonDeps{}
			var err error
			{
				caller := cf.ForInstance("Interceptors")
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/datastore",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &datastore.SingletonDeps{}
			var err error
			{
				// name: "cert"
				p := &secrets.Secret{}
				fninit.MustUnwrapProto("CgRjZXJ0", p)

				caller := cf.ForInstance("Cert")
				if deps.Cert, err = secrets.ProvideSecret(ctx, caller, p); err != nil {
					return nil, err
				}
			}

			{
				// name: "gen"
				p := &secrets.Secret{}
				fninit.MustUnwrapProto("CgNnZW4=", p)

				caller := cf.ForInstance("Gen")
				if deps.Gen, err = secrets.ProvideSecret(ctx, caller, p); err != nil {
					return nil, err
				}
			}

			{
				// name: "keygen"
				p := &secrets.Secret{}
				fninit.MustUnwrapProto("CgZrZXlnZW4=", p)

				caller := cf.ForInstance("Keygen")
				if deps.Keygen, err = secrets.ProvideSecret(ctx, caller, p); err != nil {
					return nil, err
				}
			}

			{
				caller := cf.ForInstance("ReadinessCheck")
				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/post",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &post.ServiceDeps{}
			var err error
			{
				// configuration: {
				//   service_name: "PostService"
				//   method_name: "*"
				//   maximum_deadline: 5
				// }
				p := &deadlines.Deadline{}
				fninit.MustUnwrapProto("ChUKC1Bvc3RTZXJ2aWNlEgEqHQAAoEA=", p)

				caller := cf.ForInstance("Dl")
				singletonDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/grpc/deadlines", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Dl, err = deadlines.ProvideDeadlines(ctx, caller, p, singletonDeps.(*deadlines.SingletonDeps)); err != nil {
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
				fninit.MustUnwrapProto("CgRtYWluEh4KCnNjaGVtYS50eHQSEGp1c3QgYSB0ZXN0IGZpbGU=", p)

				caller := cf.ForInstance("Main")
				singletonDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", "SingletonDeps")
				if err != nil {
					return nil, err
				}
				if deps.Main, err = datastore.ProvideDatabase(ctx, caller, p, singletonDeps.(*datastore.SingletonDeps)); err != nil {
					return nil, err
				}
			}

			{
				// package_name: "namespacelabs.dev/foundation/std/testdata/service/simple"
				p := &grpc.Backend{}
				fninit.MustUnwrapProto("CjhuYW1lc3BhY2VsYWJzLmRldi9mb3VuZGF0aW9uL3N0ZC90ZXN0ZGF0YS9zZXJ2aWNlL3NpbXBsZQ==", p)

				caller := cf.ForInstance("SimpleConn")
				if deps.SimpleConn, err = grpc.ProvideConn(ctx, caller, p); err != nil {
					return nil, err
				}

				deps.Simple = simple.NewEmptyServiceClient(deps.SimpleConn)
			}
			return deps, err
		},
	})

	di.Add(fninit.Factory{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Typename:    "SingletonDeps",
		Do: func(ctx context.Context, cf *fninit.CallerFactory) (interface{}, error) {
			deps := &logging.SingletonDeps{}
			var err error
			{
				caller := cf.ForInstance("Interceptors")
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, caller, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.AddInitializer(fninit.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", "SingletonDeps")
			if err != nil {
				return err
			}
			return metrics.Prepare(ctx, singletonDeps.(*metrics.SingletonDeps))
		},
	})

	di.AddInitializer(fninit.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", "SingletonDeps")
			if err != nil {
				return err
			}
			return tracing.Prepare(ctx, singletonDeps.(*tracing.SingletonDeps))
		},
	})

	di.AddInitializer(fninit.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/grpc/deadlines", "SingletonDeps")
			if err != nil {
				return err
			}
			return deadlines.Prepare(ctx, singletonDeps.(*deadlines.SingletonDeps))
		},
	})

	di.AddInitializer(fninit.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Do: func(ctx context.Context) error {
			singletonDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/grpc/logging", "SingletonDeps")
			if err != nil {
				return err
			}
			return logging.Prepare(ctx, singletonDeps.(*logging.SingletonDeps))
		},
	})

	server = &ServerDeps{}

	postDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", "ServiceDeps")
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
