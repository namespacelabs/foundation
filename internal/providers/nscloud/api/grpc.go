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
