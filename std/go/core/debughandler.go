// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package core

import (
	"context"
	"net/http"
	"sync"

	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/core/types"
)

var debugHandlers struct {
	mu       sync.RWMutex
	handlers map[string]http.Handler
}

type DebugHandler struct {
	owner schema.PackageName
}

func (h DebugHandler) Handle(handler http.Handler) {
	debugHandlers.mu.Lock()
	defer debugHandlers.mu.Unlock()

	if debugHandlers.handlers == nil {
		debugHandlers.handlers = map[string]http.Handler{}
	}

	debugHandlers.handlers[h.owner.String()] = handler
}

func ProvideDebugHandler(ctx context.Context, _ *types.DebugHandlerArgs) (DebugHandler, error) {
	return DebugHandler{InstantiationPathFromContext(ctx).Last()}, nil
}
