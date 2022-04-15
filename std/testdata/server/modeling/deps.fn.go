// This file was automatically generated.
// This file uses type assertions. When go 1.18 is more widely deployed, it will switch to generics.
package main

import (
	"context"

	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
	"namespacelabs.dev/foundation/std/testdata/counter"
	"namespacelabs.dev/foundation/std/testdata/counter/data"
	"namespacelabs.dev/foundation/std/testdata/service/multicounter"
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

	namespacelabs_dev_foundation_std_testdata_counter__Counter = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/counter",
		Typename:    "Counter",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps counter.CounterDeps
			var err error

			if deps.Data, err = data.ProvideData(ctx, nil); err != nil {
				return nil, err
			}

			return deps, nil
		},
	}

	namespacelabs_dev_foundation_std_testdata_service_multicounter = core.Provider{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/multicounter",
		Instantiate: func(ctx context.Context, di core.Dependencies) (interface{}, error) {
			var deps multicounter.ServiceDeps
			var err error

			err = di.Instantiate(ctx, namespacelabs_dev_foundation_std_testdata_counter__Counter, func(ctx context.Context, scoped interface{}) (err error) {
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

			err = di.Instantiate(ctx, namespacelabs_dev_foundation_std_testdata_counter__Counter, func(ctx context.Context, scoped interface{}) (err error) {
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

	if err := depgraph.Instantiate(ctx, namespacelabs_dev_foundation_std_testdata_service_multicounter, func(ctx context.Context, v interface{}) error {
		multicounter.WireService(ctx, srv.Scope(namespacelabs_dev_foundation_std_testdata_service_multicounter.PackageName), v.(multicounter.ServiceDeps))
		return nil
	}); err != nil {
		errs = append(errs, err)
	}

	return errs
}
