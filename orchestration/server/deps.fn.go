// This file was automatically generated by Namespace.
// DO NOT EDIT. To update, re-run `ns generate`.

package main

import (
	"context"
	"namespacelabs.dev/foundation/orchestration/controllers"
	"namespacelabs.dev/foundation/orchestration/legacycontroller"
	"namespacelabs.dev/foundation/orchestration/service"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/metrics"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/grpc/logging"
)

func RegisterInitializers(di *core.DependencyGraph) {
	di.AddInitializers(metrics.Initializers__so2f3v...)
	di.AddInitializers(legacycontroller.Initializers__onl1mt...)
	di.AddInitializers(logging.Initializers__16bc0q...)
}

func WireServices(ctx context.Context, srv server.Server, depgraph core.Dependencies) []error {
	var errs []error

	if err := depgraph.Instantiate(ctx, controllers.Provider__6f40u5, func(ctx context.Context, v interface{}) error {
		controllers.WireService(ctx, srv.Scope(controllers.Package__6f40u5), v.(controllers.ServiceDeps))
		return nil
	}); err != nil {
		errs = append(errs, err)
	}

	if err := depgraph.Instantiate(ctx, service.Provider__v9aee7, func(ctx context.Context, v interface{}) error {
		service.WireService(ctx, srv.Scope(service.Package__v9aee7), v.(service.ServiceDeps))
		return nil
	}); err != nil {
		errs = append(errs, err)
	}

	return errs
}
