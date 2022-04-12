// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deploy

import (
	"context"
	"fmt"
	"time"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/console/renderwait"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/provision/startup"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const fetchLogsAfter = 10 * time.Second

func Wait(ctx context.Context, env ops.Environment, servers []*schema.Server, waiters []ops.Waiter) error {
	timer := time.AfterFunc(fetchLogsAfter, func() {
		fmt.Fprintf(console.TypedOutput(ctx, "deploy", tasks.CatOutputUs), "Deployment is taking a long time. Fetching logs:\n")
		startup.FetchLogs(ctx, env, servers)
	})
	defer timer.Stop()

	rwb := renderwait.NewBlock(ctx, "deploy")
	err := ops.WaitMultiple(ctx, waiters, rwb.Ch())

	// Make sure that rwb completes before further output below (for ordering purposes).
	if waitErr := rwb.Wait(ctx); waitErr != nil {
		if err == nil {
			err = waitErr
		}
	}

	return err
}
