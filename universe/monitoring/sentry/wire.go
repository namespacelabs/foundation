// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package sentry

import (
	"context"

	"github.com/getsentry/sentry-go"
)

func Prepare(ctx context.Context, deps ExtensionDeps) error {
	if err := sentry.Init(sentry.ClientOptions{
		Dsn:         string(deps.Dsn.MustValue()),
		ServerName:  deps.ServerInfo.ServerName,
		Environment: deps.ServerInfo.EnvName,
		Release:     deps.ServerInfo.GetVcs().GetRevision(),
	}); err != nil {
		return err
	}

	return nil
}
