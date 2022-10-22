// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package pkggraph

import (
	"context"

	"namespacelabs.dev/foundation/schema"
)

func LoadBinary(ctx context.Context, pl PackageLoader, ref *schema.PackageRef) (*Package, *schema.Binary, error) {
	binPkg, err := pl.LoadByName(ctx, ref.AsPackageName())
	if err != nil {
		return nil, nil, err
	}

	binary, err := binPkg.LookupBinary(ref.Name)
	if err != nil {
		return nil, nil, err
	}

	return binPkg, binary, nil
}
