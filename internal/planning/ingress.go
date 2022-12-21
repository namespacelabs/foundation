// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package planning

import "namespacelabs.dev/foundation/schema"

func IngressOwnedBy(allFragments []*schema.IngressFragment, srv schema.PackageName) []*schema.IngressFragment {
	var frags []*schema.IngressFragment
	for _, fr := range allFragments {
		if srv.Equals(fr.Owner) {
			frags = append(frags, fr)
		}
	}
	return frags
}
