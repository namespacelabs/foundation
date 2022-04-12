// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package startup

import (
	"context"
	"fmt"
	"time"

	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/engine/ops"
	"namespacelabs.dev/foundation/runtime"
	"namespacelabs.dev/foundation/schema"
)

const streamLogTimeout = time.Second

func FetchLogs(ctx context.Context, env ops.Environment, servers []*schema.Server) {
	rt := runtime.For(env)

	for _, srv := range servers {
		err := rt.Observe(ctx, srv, runtime.ObserveOpts{OneShot: true}, func(ev runtime.ObserveEvent) error {
			w := console.Output(ctx, ev.HumanReadableID)
			ctx, cancel := context.WithTimeout(ctx, streamLogTimeout)
			defer cancel()

			return rt.StreamLogsTo(ctx, w, srv, runtime.StreamLogsOpts{
				InstanceID: ev.InstanceID,
				TailLines:  20,
				Follow:     false,
			})
		})

		if err != nil {
			fmt.Fprintf(console.Warnings(ctx), "%s: failed to obtain logs: %v", srv.PackageName, err)
		}
	}
}
