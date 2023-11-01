// This file was automatically generated by Namespace.
// DO NOT EDIT. To update, re-run `ns generate`.

package k8sdetector

import (
	"context"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/monitoring/tracing"
)

// Dependencies that are instantiated once for the lifetime of the extension.
type ExtensionDeps struct {
	Detector tracing.Detector
}

var (
	Package__aar2v4 = &core.Package{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing/otel/k8sdetector",
	}

	Provider__aar2v4 = core.Provider{
		Package:     Package__aar2v4,
		Instantiate: makeDeps__aar2v4,
	}

	Initializers__aar2v4 = []*core.Initializer{
		{
			Package: Package__aar2v4,
			Do: func(ctx context.Context, di core.Dependencies) error {
				return di.Instantiate(ctx, Provider__aar2v4, func(ctx context.Context, v interface{}) error {
					return Prepare(ctx, v.(ExtensionDeps))
				})
			},
		},
	}
)

func makeDeps__aar2v4(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ExtensionDeps

	if err := di.Instantiate(ctx, tracing.Provider__70o2mm, func(ctx context.Context, v interface{}) (err error) {
		// name: "kubernetes"
		if deps.Detector, err = tracing.ProvideDetector(ctx, core.MustUnwrapProto("CgprdWJlcm5ldGVz", &tracing.DetectorArgs{}).(*tracing.DetectorArgs), v.(tracing.ExtensionDeps)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return deps, nil
}
