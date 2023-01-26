// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
