// This file was automatically generated by Namespace.
// DO NOT EDIT. To update, re-run `ns generate`.

package fetch

import (
	"context"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/std/go/server"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
)

// Dependencies that are instantiated once for the lifetime of the service.
type ServiceDeps struct {
	Dl *deadlines.DeadlineRegistration
}

// Verify that WireService is present and has the appropriate type.
type checkWireService func(context.Context, server.Registrar, ServiceDeps)

var _ checkWireService = WireService

var (
	Package__i0mog6 = &core.Package{
		PackageName: "namespacelabs.dev/foundation/std/testdata/service/fetch",
	}

	Provider__i0mog6 = core.Provider{
		Package:     Package__i0mog6,
		Instantiate: makeDeps__i0mog6,
	}
)

func makeDeps__i0mog6(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ServiceDeps

	if err := di.Instantiate(ctx, deadlines.Provider__vbko45, func(ctx context.Context, v interface{}) (err error) {
		// configuration: {
		//   service_name: "PostService"
		//   method_name: "Fetch"
		//   maximum_deadline: 0.5
		// }
		if deps.Dl, err = deadlines.ProvideDeadlines(ctx, core.MustUnwrapProto("ChkKC1Bvc3RTZXJ2aWNlEgVGZXRjaB0AAAA/", &deadlines.Deadline{}).(*deadlines.Deadline), v.(deadlines.ExtensionDeps)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return deps, nil
}
