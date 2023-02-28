// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"encoding/json"

	"namespacelabs.dev/foundation/internal/auth"
	"namespacelabs.dev/foundation/internal/environment"
	"namespacelabs.dev/foundation/internal/github"
	nscloudpb "namespacelabs.dev/foundation/schema/nscloud"
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

	if !environment.IsRunningInCI() {
		user, err := auth.LoadUser()
		if err != nil {
			return nil, err
		}

		attach := &nscloudpb.CreatorAttachment{
			Username: user.Username,
		}
		content, err := json.Marshal(attach)
		if err != nil {
			return nil, err
		}

		res = append(res, Attachment{
			TypeURL: "namespacelabs.dev/foundation/schema.nscloud.CreatorAttachment",
			Content: content,
		})
	}

	return res, nil
}
