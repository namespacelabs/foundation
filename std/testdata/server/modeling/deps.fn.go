// This file was automatically generated.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/grpc/server"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	"namespacelabs.dev/foundation/std/testdata/counter"
	"namespacelabs.dev/foundation/std/testdata/counter/data"
	"namespacelabs.dev/foundation/std/testdata/service/multicounter"
)

type ServerDeps struct {
	multicounter multicounter.ServiceDeps
}

// This code uses type assertions for now. When go 1.18 is more widely deployed, it will switch to generics.
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
		Package: "namespacelabs.dev/foundation/std/testdata/counter",
		Scope:   "Counter",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps counter.CounterDeps
			var err error

			if deps.Data, err = data.ProvideData(ctx, nil); err != nil {
				return nil, err
			}

			return deps, err
		},
	})

	di.Add(core.Provider{
		Package: "namespacelabs.dev/foundation/std/testdata/service/multicounter",
		Do: func(ctx context.Context) (interface{}, error) {
			var deps multicounter.ServiceDeps
			var err error

			err = di.Instantiate(ctx, core.Reference{
				Package: "namespacelabs.dev/foundation/std/testdata/counter",
				Scope:   "Counter"},
				func(ctx context.Context, scoped interface{}) (err error) {
					// name: "one"
					p := &counter.Input{}
					core.MustUnwrapProto("CgNvbmU=", p)

					if deps.One, err = counter.ProvideCounter(ctx, p, scoped.(counter.CounterDeps)); err != nil {
						return err
					}
					return nil
				})
			if err != nil {
				return nil, err
			}

			err = di.Instantiate(ctx, core.Reference{
				Package: "namespacelabs.dev/foundation/std/testdata/counter",
				Scope:   "Counter"},
				func(ctx context.Context, scoped interface{}) (err error) {
					// name: "two"
					p := &counter.Input{}
					core.MustUnwrapProto("CgN0d28=", p)

					if deps.Two, err = counter.ProvideCounter(ctx, p, scoped.(counter.CounterDeps)); err != nil {
						return err
					}
					return nil
				})
			if err != nil {
				return nil, err
			}

			return deps, err
		},
	})

	server = &ServerDeps{}
	di.AddInitializer(core.Initializer{
		PackageName: "namespacelabs.dev/foundation/std/testdata/server/modeling",
		Do: func(ctx context.Context) error {

			err = di.Instantiate(ctx, core.Reference{Package: "namespacelabs.dev/foundation/std/testdata/service/multicounter"},
				func(ctx context.Context, v interface{}) (err error) {
					server.multicounter = v.(multicounter.ServiceDeps)
					return nil
				})
			if err != nil {
				return err
			}

			return nil
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

	return server, di.Init(ctx)
}

func WireServices(ctx context.Context, srv *server.Grpc, server *ServerDeps) {
	multicounter.WireService(ctx, srv, server.multicounter)
}
