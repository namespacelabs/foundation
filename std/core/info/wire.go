// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package info

import (
	"context"

	"namespacelabs.dev/foundation/std/core/types"
	"namespacelabs.dev/foundation/std/go/core"
)

type ServerInfoArgs = types.ServerInfoArgs
type ServerInfo = types.ServerInfo

func ProvideServerInfo(ctx context.Context, args *ServerInfoArgs) (*ServerInfo, error) {
	return core.ProvideServerInfo(ctx, args)
}
