// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devbox

import (
	"context"
	"fmt"
	"io"

	devboxv1beta "buf.build/gen/go/namespace/cloud/protocolbuffers/go/proto/namespace/private/devbox"
	"github.com/kr/text"
	"namespacelabs.dev/foundation/internal/console"
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
	if fnapi.DebugApiResponse {
		fmt.Fprintf(console.Debug(ctx), "Response Body: %v\n", resp)
	}

	devbox := findDevboxByTag(resp.DevBoxes, tag)
	if devbox == nil {
		return nil, fmt.Errorf("devbox '%s' not found", tag)
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

type devboxInstance struct {
	regionalInstanceId  string
	regionalSshEndpoint string
}

func doEnsureDevbox(ctx context.Context, devboxClient *private.DevBoxServiceClient, tag string) (devboxInstance, error) {
	resp, err := devboxClient.EnsureDevBox(ctx, &devboxv1beta.EnsureDevBoxRequest{DevboxTag: tag})
	if err != nil {
		return devboxInstance{}, err
	}
	if fnapi.DebugApiResponse {
		fmt.Fprintf(console.Debug(ctx), "Response Body: %v\n", resp)
	}

	return devboxInstance{
		regionalInstanceId:  resp.RegionalInstanceId,
		regionalSshEndpoint: resp.RegionalSshEndpoint,
	}, nil
}

func indent(w io.Writer) io.Writer {
	return text.NewIndentWriter(w, []byte("    "))
}
