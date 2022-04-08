// This file was automatically generated.
package deadlines

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

type ExtensionDeps struct {
	Interceptors interceptors.Registration
}

type _checkProvideDeadlines func(context.Context, string, *Deadline, ExtensionDeps) (*DeadlineRegistration, error)

var _ _checkProvideDeadlines = ProvideDeadlines

type _checkPrepare func(context.Context, ExtensionDeps) error

var _ _checkPrepare = Prepare
