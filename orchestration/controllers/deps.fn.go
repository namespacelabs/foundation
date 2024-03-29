// This file was automatically generated by Namespace.
// DO NOT EDIT. To update, re-run `ns generate`.

package controllers

import (
	"context"
	fncore "namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/server"
)

// Dependencies that are instantiated once for the lifetime of the service.
type ServiceDeps struct {
	Ready core.Check
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, server.Registrar, ServiceDeps)

var _ checkWireService = WireService

var (
	Package__6f40u5 = &core.Package{
		PackageName: "namespacelabs.dev/foundation/orchestration/controllers",
	}

	Provider__6f40u5 = core.Provider{
		Package:     Package__6f40u5,
		Instantiate: makeDeps__6f40u5,
	}
)

func makeDeps__6f40u5(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ServiceDeps

	if deps.Ready, err = fncore.ProvideReadinessCheck(ctx, nil); err != nil {
		return nil, err
	}

	return deps, nil
}
