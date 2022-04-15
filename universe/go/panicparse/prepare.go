// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package panicparse

import (
	"context"
	"net/http"

	"github.com/maruel/panicparse/v2/stack/webstack"
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	deps.DebugHandler.Handle(http.HandlerFunc(webstack.SnapshotHandler))
	return nil
}
