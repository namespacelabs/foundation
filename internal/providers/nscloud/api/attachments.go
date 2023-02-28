// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"encoding/json"

	"namespacelabs.dev/foundation/internal/github"
)

func clusterAttachments(ctx context.Context) ([]Attachment, error) {
	var res []Attachment

	if github.IsRunningInActions() {
		attach, err := github.AttachmentFromEnv(ctx)
		if err != nil {
			return nil, err
		}

		content, err := json.Marshal(attach)
		if err != nil {
			return nil, err
		}

		res = append(res, Attachment{
			TypeURL: "namespacelabs.dev/foundation/schema.GithubAttachment",
			Content: content,
		})
	}

	if github.IsRunningInActions() {
		attach, err := github.AttachmentFromEnv(ctx)
		if err != nil {
			return nil, err
		}

		content, err := json.Marshal(attach)
		if err != nil {
			return nil, err
		}

		res = append(res, Attachment{
			TypeURL: "namespacelabs.dev/foundation/schema.GithubAttachment",
			Content: content,
		})
	}

	return res, nil
}
