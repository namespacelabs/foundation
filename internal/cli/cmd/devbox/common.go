// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"fmt"

	devboxv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/private/devbox"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/providers/nscloud/api/private"
)

func getDevBoxClient(ctx context.Context) (*private.DevBoxServiceClient, error) {
	token, err := fnapi.IssueBearerToken(ctx)
	if err != nil {
		return nil, err
	}

	return private.MakeDevBoxClient(ctx, token)
}

func getSingleDevbox(ctx context.Context, devboxClient *private.DevBoxServiceClient, tag string) (*devboxv1beta.DevBox, error) {
	resp, err := devboxClient.ListDevBoxes(ctx, &devboxv1beta.ListDevBoxesRequest{
		TagFilter: []string{tag},
	})
	if err != nil {
		return nil, err
	}
	devbox := findDevboxByTag(resp.DevBoxes, tag)
	if devbox == nil {
		return nil, fmt.Errorf("devbox '" + tag + "' not found")
	}
	return devbox, nil
}

func findDevboxByTag(devboxes []*devboxv1beta.DevBox, tag string) *devboxv1beta.DevBox {
	for _, candidate := range devboxes {
		if candidate.GetDevboxSpec().GetTag() == tag {
			return candidate
		}
	}

	return nil
}
