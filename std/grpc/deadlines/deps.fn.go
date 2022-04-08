// This file was automatically generated.
package deadlines

import (
	"context"

	fninit "namespacelabs.dev/foundation/std/go/core/init"
	"namespacelabs.dev/foundation/std/go/grpc/interceptors"
)

type SingletonDeps struct {
	Interceptors interceptors.Registration
}

type _checkProvideDeadlines func(context.Context, fninit.Caller, *Deadline, *SingletonDeps) (*DeadlineRegistration, error)

var _ _checkProvideDeadlines = ProvideDeadlines

type _checkPrepare func(context.Context, *SingletonDeps) error

var _ _checkPrepare = Prepare
