// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package invariants

import (
	"context"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type PackageNameLike interface {
	GetPackageName() string
}

func EnsurePackageLoaded(ctx context.Context, pl pkggraph.PackageLoader, owner PackageNameLike, target PackageNameLike) error {
	// We allow a nil pl because this is also used in phase1 + phase2 where there's no pl.
	t := target.GetPackageName()
	if pl != nil && t != owner.GetPackageName() {
		if err := pl.Ensure(ctx, schema.PackageName(t)); err != nil {
			return err
		}
	}

	return nil
}
