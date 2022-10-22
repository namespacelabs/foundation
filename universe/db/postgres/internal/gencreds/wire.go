// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package gencreds

import (
	"context"
)

func ProvideCreds(ctx context.Context, _ *CredsRequest, deps ExtensionDeps) (*Creds, error) {
	creds := &Creds{
		Password: string(deps.Password.MustValue()),
	}
	return creds, nil
}
