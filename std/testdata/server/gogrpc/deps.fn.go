// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/schema"
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

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &metrics.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Interceptors").WithContext(ctx)
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &tracing.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Interceptors").WithContext(ctx)
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &deadlines.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Interceptors").WithContext(ctx)
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/datastore",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &datastore.ExtensionDeps{}
			var err error
			{
				// name: "cert"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgRjZXJ0", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "Cert").WithContext(ctx)
				if deps.Cert, err = secrets.ProvideSecret(ctx, p); err != nil {
					return nil, err
				}
			}

			{
				// name: "gen"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgNnZW4=", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "Gen").WithContext(ctx)
				if deps.Gen, err = secrets.ProvideSecret(ctx, p); err != nil {
					return nil, err
				}
			}

			{
				// name: "keygen"
				p := &secrets.Secret{}
				core.MustUnwrapProto("CgZrZXlnZW4=", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "Keygen").WithContext(ctx)
				if deps.Keygen, err = secrets.ProvideSecret(ctx, p); err != nil {
					return nil, err
				}
			}

			{
				ctx = core.PathFromContext(ctx).Append(pkg, "ReadinessCheck").WithContext(ctx)
				if deps.ReadinessCheck, err = core.ProvideReadinessCheck(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/post",
		Typename:    "ServiceDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
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

				ctx = core.PathFromContext(ctx).Append(pkg, "Dl").WithContext(ctx)
				extensionDeps, err := di.GetSingleton(ctx,
					"namespacelabs.dev/foundation/std/grpc/deadlines", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Dl, err = deadlines.ProvideDeadlines(ctx, p, extensionDeps.(*deadlines.ExtensionDeps)); err != nil {
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

				ctx = core.PathFromContext(ctx).Append(pkg, "Main").WithContext(ctx)
				extensionDeps, err := di.GetSingleton(ctx,
					"namespacelabs.dev/foundation/std/testdata/datastore", "ExtensionDeps")
				if err != nil {
					return nil, err
				}
				if deps.Main, err = datastore.ProvideDatabase(ctx, p, extensionDeps.(*datastore.ExtensionDeps)); err != nil {
					return nil, err
				}
			}

			{
				// package_name: "namespacelabs.dev/foundation/std/testdata/service/simple"
				p := &grpc.Backend{}
				core.MustUnwrapProto("CjhuYW1lc3BhY2VsYWJzLmRldi9mb3VuZGF0aW9uL3N0ZC90ZXN0ZGF0YS9zZXJ2aWNlL3NpbXBsZQ==", p)

				ctx = core.PathFromContext(ctx).Append(pkg, "SimpleConn").WithContext(ctx)
				if deps.SimpleConn, err = grpc.ProvideConn(ctx, p); err != nil {
					return nil, err
				}

				deps.Simple = simple.NewEmptyServiceClient(deps.SimpleConn)
			}
			return deps, err
		},
	})

	di.Add(core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Typename:    "ExtensionDeps",
		Do: func(ctx context.Context, pkg schema.PackageName) (interface{}, error) {
			deps := &logging.ExtensionDeps{}
			var err error
			{
				ctx = core.PathFromContext(ctx).Append(pkg, "Interceptors").WithContext(ctx)
				if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
					return nil, err
				}
			}
			return deps, err
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/go/grpc/metrics",
		Do: func(ctx context.Context) error {
			extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/go/grpc/metrics", "ExtensionDeps")
			if err != nil {
				return err
			}
			return metrics.Prepare(ctx, extensionDeps.(*metrics.ExtensionDeps))
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
		Do: func(ctx context.Context) error {
			extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/monitoring/tracing", "ExtensionDeps")
			if err != nil {
				return err
			}
			return tracing.Prepare(ctx, extensionDeps.(*tracing.ExtensionDeps))
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/deadlines",
		Do: func(ctx context.Context) error {
			extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/grpc/deadlines", "ExtensionDeps")
			if err != nil {
				return err
			}
			return deadlines.Prepare(ctx, extensionDeps.(*deadlines.ExtensionDeps))
		},
	})

	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/grpc/logging",
		Do: func(ctx context.Context) error {
			extensionDeps, err := di.GetSingleton(ctx, "namespacelabs.dev/foundation/std/grpc/logging", "ExtensionDeps")
			if err != nil {
				return err
			}
			return logging.Prepare(ctx, extensionDeps.(*logging.ExtensionDeps))
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
