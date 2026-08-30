// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
