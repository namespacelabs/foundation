package core

import (
	"context"
	"sync"

	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/schema"
)

var debugHandlers struct {
	mu       sync.RWMutex
	handlers map[string]*mux.Router
}

type DebugHandler struct {
	owner schema.PackageName
}

func (h DebugHandler) Mux() *mux.Router {
	debugHandlers.mu.Lock()
	defer debugHandlers.mu.Unlock()

	if debugHandlers.handlers == nil {
		debugHandlers.handlers = map[string]*mux.Router{}
	}

	if r, ok := debugHandlers.handlers[h.owner.String()]; ok {
		return r
	}

	r := mux.NewRouter()
	debugHandlers.handlers[h.owner.String()] = r
	return r
}

func ProvideDebugHandler(ctx context.Context, _ *DebugHandlerArgs) (DebugHandler, error) {
	return DebugHandler{InstantiationPathFromContext(ctx).Last()}, nil
}
