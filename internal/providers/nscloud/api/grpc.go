// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"fmt"

	"google.golang.org/grpc/metadata"
	"namespacelabs.dev/foundation/internal/fnapi"
)

func ContextWithBearerToken(ctx context.Context) (context.Context, error) {
	token, err := fnapi.IssueBearerToken(ctx)
	if err != nil {
		return nil, err
	}

	return metadata.AppendToOutgoingContext(ctx, "authorization", fmt.Sprintf("Bearer %s", token.BearerToken)), nil
}
