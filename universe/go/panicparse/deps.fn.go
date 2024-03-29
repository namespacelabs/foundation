// This file was automatically generated by Namespace.
// DO NOT EDIT. To update, re-run `ns generate`.

package panicparse

import (
	"context"
	fncore "namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/std/go/core"
)

// Dependencies that are instantiated once for the lifetime of the extension.
type ExtensionDeps struct {
	DebugHandler core.DebugHandler
}

var (
	Package__99b5nh = &core.Package{
		PackageName: "namespacelabs.dev/foundation/universe/go/panicparse",
	}

	Provider__99b5nh = core.Provider{
		Package:     Package__99b5nh,
		Instantiate: makeDeps__99b5nh,
	}

	Initializers__99b5nh = []*core.Initializer{
		{
			Package: Package__99b5nh,
			Do: func(ctx context.Context, di core.Dependencies) error {
				return di.Instantiate(ctx, Provider__99b5nh, func(ctx context.Context, v interface{}) error {
					return Prepare(ctx, v.(ExtensionDeps))
				})
			},
		},
	}
)

func makeDeps__99b5nh(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ExtensionDeps

	if deps.DebugHandler, err = fncore.ProvideDebugHandler(ctx, nil); err != nil {
		return nil, err
	}

	return deps, nil
}
