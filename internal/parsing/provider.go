// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package parsing

import (
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func FindProvider(pkg *pkggraph.Package, packageName schema.PackageName, typeName string) (*schema.Node, *schema.Provides) {
	// Only extensions can be providers.
	if n := pkg.Extension; n != nil {
		if packageName.Equals(n.GetPackageName()) {
			for _, p := range n.Provides {
				if p.Name == typeName {
					return n, p
				}
			}
		}
	}

	return nil, nil
}
