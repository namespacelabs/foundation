// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package wscontents

import (
	"context"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/std/tasks"
)

func ChangeTrigger(absPath, rel string) compute.Computable[compute.Versioned] {
	return compute.Inline(tasks.Action("module.contents.observe"), func(ctx context.Context) (compute.Versioned, error) {
		return makeVersioned(ctx, absPath, rel, true, true, nil)
	})
}
