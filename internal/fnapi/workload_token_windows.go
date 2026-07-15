// Copyright 2026 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package fnapi

import "namespacelabs.dev/foundation/internal/providers/nscloud/metadata"

func defaultWorkloadTokenPath() (string, error) {
	return metadata.TokenFile()
}
