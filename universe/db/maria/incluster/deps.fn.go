// This file was automatically generated by Foundation.
// DO NOT EDIT. To update, re-run `fn generate`.

package incluster

import (
	"context"
	"database/sql"
	fncore "namespacelabs.dev/foundation/std/core"
	"namespacelabs.dev/foundation/std/go/core"
	"namespacelabs.dev/foundation/universe/db/maria/incluster/creds"
)

// Dependencies that are instantiated once for the lifetime of the extension.
type ExtensionDeps struct {
	Creds          *creds.Creds
	ReadinessCheck core.Check
}

type _checkProvideDatabase func(context.Context, *Database, ExtensionDeps) (*sql.DB, error)

var _ _checkProvideDatabase = ProvideDatabase

var (
	Package__r7qsle = &core.Package{
		PackageName: "namespacelabs.dev/foundation/universe/db/maria/incluster",
	}

	Provider__r7qsle = core.Provider{
		Package:     Package__r7qsle,
		Instantiate: makeDeps__r7qsle,
	}
)

func makeDeps__r7qsle(ctx context.Context, di core.Dependencies) (_ interface{}, err error) {
	var deps ExtensionDeps

	if err := di.Instantiate(ctx, creds.Provider__bihnv9, func(ctx context.Context, v interface{}) (err error) {
		if deps.Creds, err = creds.ProvideCreds(ctx, nil, v.(creds.ExtensionDeps)); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if deps.ReadinessCheck, err = fncore.ProvideReadinessCheck(ctx, nil); err != nil {
		return nil, err
	}

	return deps, nil
}
