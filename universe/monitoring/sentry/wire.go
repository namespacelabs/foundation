package sentry

import (
	"context"

	"github.com/getsentry/sentry-go"
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	return sentry.Init(sentry.ClientOptions{
		Dsn:         string(deps.Dsn.MustValue()),
		ServerName:  deps.ServerInfo.ServerName,
		Environment: deps.ServerInfo.EnvName,
	})
}
