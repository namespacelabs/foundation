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
	Package__27cml6 = &core.Package{
		PackageName:         "namespacelabs.dev/foundation/universe/otel/k8sdetector",
		PackageDependencies: []string{"namespacelabs.dev/foundation/std/monitoring/tracing"},
	}

	Provider__27cml6 = core.Provider{
		Package:     Package__27cml6,
		Instantiate: makeDeps__27cml6,
	}

	Initializers__27cml6 = []*core.Initializer{
		{
			Package: Package__27cml6,
			Do: func(ctx context.Context, di core.Dependencies) error {
				return di.Instantiate(ctx, Provider__27cml6, func(ctx context.Context, v interface{}) error {
					return Prepare(ctx, v.(ExtensionDeps))
				})
			},
		},
	}
)

func makeDeps__27cml6(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
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
