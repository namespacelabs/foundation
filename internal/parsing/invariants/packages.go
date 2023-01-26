package invariants

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func EnsurePackageLoaded(ctx context.Context, pl pkggraph.PackageLoader, owner schema.PackageName, ref *schema.PackageRef) error {
	// We allow a nil pl because this is also used in phase1 + phase2 where there's no pl.
	if pl != nil && ref.AsPackageName() != owner {
		if _, err := pl.LoadByName(ctx, ref.AsPackageName()); err != nil {
			return err
		}
	}

	return nil
}
