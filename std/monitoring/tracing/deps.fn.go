// This file was automatically generated by Foundation.
// DO NOT EDIT. To update, re-run `fn generate`.

package tracing

import (
	"context"
	fncore "namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/std/core/types"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
	"namespacelabs.dev/foundation/std/go/http/middleware"
)

// Dependencies that are instantiated once for the lifetime of the extension.
type ExtensionDeps struct {
	Interceptors interceptors.Registration
	Middleware   middleware.Middleware
	ServerInfo   *types.ServerInfo
}

type _checkProvideExporter func(context.Context, *ExporterArgs, ExtensionDeps) (Exporter, error)

var _ _checkProvideExporter = ProvideExporter

type _checkProvideHttpClientProvider func(context.Context, *NoArgs, ExtensionDeps) (HttpClientProvider, error)

var _ _checkProvideHttpClientProvider = ProvideHttpClientProvider

type _checkProvideTracerProvider func(context.Context, *NoArgs, ExtensionDeps) (DeferredTracerProvider, error)

var _ _checkProvideTracerProvider = ProvideTracerProvider

var (
	Package__70o2mm = &core.Package{
		PackageName: "namespacelabs.dev/foundation/std/monitoring/tracing",
	}

	Provider__70o2mm = core.Provider{
		Package:     Package__70o2mm,
		Instantiate: makeDeps__70o2mm,
	}

	Initializers__70o2mm = []*core.Initializer{
		{
			Package: Package__70o2mm,
			Do: func(ctx context.Context, di core.Dependencies) error {
				return di.Instantiate(ctx, Provider__70o2mm, func(ctx context.Context, v interface{}) error {
					return Prepare(ctx, v.(ExtensionDeps))
				})
			},
		},
	}
)

func makeDeps__70o2mm(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ExtensionDeps

	if deps.Interceptors, err = interceptors.ProvideInterceptorRegistration(ctx, nil); err != nil {
		return nil, err
	}

	if deps.Middleware, err = middleware.ProvideMiddleware(ctx, nil); err != nil {
		return nil, err
	}

	if deps.ServerInfo, err = fncore.ProvideServerInfo(ctx, nil); err != nil {
		return nil, err
	}

	return deps, nil
}
