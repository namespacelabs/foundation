// This file was automatically generated by Namespace.
// DO NOT EDIT. To update, re-run `ns generate`.

package rawlistener

import (
	"context"
	"namespacelabs.dev/foundation/internal/testdata/counter"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/server"
)

// Dependencies that are instantiated once for the lifetime of the service.
type ServiceDeps struct {
	One *counter.Counter
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, server.Registrar, ServiceDeps)

var _ checkWireService = WireService

var (
	Package__7f1eor = &core.Package{
		PackageName:         "namespacelabs.dev/foundation/internal/testdata/service/rawlistener",
		PackageDependencies: []string{"namespacelabs.dev/foundation/internal/testdata/counter"}, ListenerConfiguration: "second",
	}

	Provider__7f1eor = core.Provider{
		Package:     Package__7f1eor,
		Instantiate: makeDeps__7f1eor,
	}
)

func makeDeps__7f1eor(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ServiceDeps

	if err := di.Instantiate(ctx, counter.Provider__mra5r6__Counter, func(ctx context.Context, scoped interface{}) (err error) {

		// name: "one"
		if deps.One, err = counter.ProvideCounter(ctx, core.MustUnwrapProto("CgNvbmU=", &counter.Input{}).(*counter.Input), scoped.(counter.CounterDeps)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return deps, nil
}
