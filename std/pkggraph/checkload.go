// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import (
	"context"

	"namespacelabs.dev/foundation/schema"
)

func ParseAndLoadRef(ctx context.Context, pl PackageLoader, loc Location, ref string) (*schema.PackageRef, error) {
	pkgRef, err := schema.ParsePackageRef(loc.PackageName, ref)
	if err != nil {
		return nil, err
	}

	if err := CheckLoad(ctx, pl, loc, pkgRef); err != nil {
		return nil, err
	}

	return pkgRef, nil
}

func CheckLoad(ctx context.Context, pl PackageLoader, loc Location, ref *schema.PackageRef) error {
	if loc.PackageName != ref.AsPackageName() {
		if _, err := pl.LoadByName(ctx, ref.AsPackageName()); err != nil {
			return err
		}
	}
	return nil
}
