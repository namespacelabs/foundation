// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"

	"namespacelabs.dev/foundation/internal/frontend/fncue"
)

func applyInputs(ctx context.Context, fetcher Fetcher, vv *fncue.CueV, recorded []fncue.KeyAndPath) (*fncue.CueV, []fncue.KeyAndPath, error) {
	var left []fncue.KeyAndPath
	for _, rec := range recorded {
		newV, err := fetcher.Fetch(ctx, vv.Val.LookupPath(rec.Target), rec)
		if err != nil {
			return nil, nil, err
		}

		if newV != nil {
			if newV != ConsumeNoValue {
				vv = vv.FillPath(rec.Target, newV)
			}
		} else {
			left = append(left, rec)
		}
	}

	return vv, left, nil
}
