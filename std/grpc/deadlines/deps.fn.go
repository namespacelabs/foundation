// This file was automatically generated.
package deadlines

import (
	"context"

	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

type SingletonDeps struct {
	Interceptors interceptors.Registration
}

type _checkProvideDeadlines func(context.Context, string, *Deadline, *SingletonDeps) (*DeadlineRegistration, error)

var _ _checkProvideDeadlines = ProvideDeadlines

type _checkPrepare func(context.Context, *SingletonDeps) error

var _ _checkPrepare = Prepare
