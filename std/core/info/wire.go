// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
