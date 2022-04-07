// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/grpc/logging"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	"namespacelabs.dev/foundation/std/secrets"
	"namespacelabs.dev/foundation/std/testdata/datastore"
	"namespacelabs.dev/foundation/std/testdata/service/post"
)

type ServerDeps struct {
	post post.ServiceDeps
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

	var datastore0 datastore.ExtensionDeps

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/secrets",
		Instance:    "datastore0",
		Do: func(ctx context.Context) (err error) {
			// name: "cert"
			p := &secrets.Secret{}
			core.MustUnwrapProto("CgRjZXJ0", p)

			if datastore0.Cert, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", p); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/secrets",
		Instance:    "datastore0",
		Do: func(ctx context.Context) (err error) {
			// name: "gen"
			p := &secrets.Secret{}
			core.MustUnwrapProto("CgNnZW4=", p)

			if datastore0.Gen, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", p); err != nil {
				return err
			}
			return nil
		},
	})

	di.Register(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/secrets",
		Instance:    "datastore0",
		Do: func(ctx context.Context) (err error) {
			// name: "keygen"
			p := &secrets.Secret{}
			core.MustUnwrapProto("CgZrZXlnZW4=", p)

			if datastore0.Keygen, err = secrets.ProvideSecret(ctx, "namespacelabs.dev/foundation/std/testdata/datastore", p); err != nil {
				return err
			}
			return nil
		},
	})

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

			if server.post.Main, err = datastore.ProvideDatabase(ctx, "namespacelabs.dev/foundation/std/testdata/service/post", p, datastore0); err != nil {
				return err
			}
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
