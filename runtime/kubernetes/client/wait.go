// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package client

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

func PollImmediateWithContext(ctx context.Context, interval, timeout time.Duration, condition wait.ConditionWithContextFunc) error {
	err := wait.PollImmediateWithContext(ctx, interval, timeout, condition)
	if err != nil {
		// The wait library never returns Cancelled, as it would break their compatibility. But we care
		// about cancelation reporting.
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return err
	}
	return nil
}
