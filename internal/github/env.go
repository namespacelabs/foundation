// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package github

import (
	"context"
	"os"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

func AttachmentFromEnv(ctx context.Context) (*schema.GithubAttachement, error) {
	requiredVars := []string{
		"GITHUB_REPOSITORY",
		"GITHUB_REPOSITORY_OWNER",
		"GITHUB_EVENT_NAME",
		"GITHUB_RUN_ID",
		"GITHUB_RUN_ATTEMPT",
		"GITHUB_SHA",
		"GITHUB_REF",
		"GITHUB_EVENT_NAME",
		"GITHUB_ACTOR",
	}
	for _, v := range requiredVars {
		if os.Getenv(v) == "" {
			return nil, fnerrors.New("missing required github enviroment variable %s", v)
		}
	}

	return &schema.GithubAttachement{
		Repository:      os.Getenv("GITHUB_REPOSITORY"),
		RepositoryOwner: os.Getenv("GITHUB_REPOSITORY_OWNER"),
		RunId:           os.Getenv("GITHUB_RUN_ID"),
		RunAttempt:      os.Getenv("GITHUB_RUN_ATTEMPT"),
		Sha:             os.Getenv("GITHUB_SHA"),
		Ref:             os.Getenv("GITHUB_REF"),
		EventName:       os.Getenv("GITHUB_EVENT_NAME"),
		Actor:           os.Getenv("GITHUB_ACTOR"),
	}, nil
}

func IsRunningInActions() bool {
	return os.Getenv("GITHUB_ACTIONS") != "true"
}
