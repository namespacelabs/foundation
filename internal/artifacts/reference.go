// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package artifacts

import "namespacelabs.dev/foundation/schema"

type Reference struct {
	URL    string
	Digest schema.Digest
}
