// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package artifacts

import "namespacelabs.dev/foundation/schema"

type Reference struct {
	URL    string
	Digest schema.Digest
}
