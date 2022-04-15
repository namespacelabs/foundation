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
