// This file was automatically generated by Foundation.
// DO NOT EDIT. To update, re-run `fn generate`.

package jaeger

import (
	"context"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

// Dependencies that are instantiated once for the lifetime of the extension.
type ExtensionDeps struct {
	OpenTelemetry tracing.Exporter
}

var (
	Package__33brri = &core.Package{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing/jaeger",
	}

	Provider__33brri = core.Provider{
		Package:     Package__33brri,
		Instantiate: makeDeps__33brri,
	}

	Initializers__33brri = []*core.Initializer{
		{
			Package: Package__33brri,
			Before:  []string{"namespacelabs.dev/foundation/std/monitoring/tracing"},
			Do: func(ctx context.Context, di core.Dependencies) error {
				return di.Instantiate(ctx, Provider__33brri, func(ctx context.Context, v interface{}) error {
					return Prepare(ctx, v.(ExtensionDeps))
				})
			},
		},
	}
)

func makeDeps__33brri(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ExtensionDeps

	if err := di.Instantiate(ctx, tracing.Provider__70o2mm, func(ctx context.Context, v interface{}) (err error) {
		// name: "jaeger"
		if deps.OpenTelemetry, err = tracing.ProvideExporter(ctx, core.MustUnwrapProto("CgZqYWVnZXI=", &tracing.ExporterArgs{}).(*tracing.ExporterArgs), v.(tracing.ExtensionDeps)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return deps, nil
}
