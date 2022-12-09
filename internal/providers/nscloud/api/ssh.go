// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/fnapi"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/std/tasks"
)

func fetchGithubSshKeys(ctx context.Context, username string) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://github.com/%s.keys", username), nil)
	if err != nil {
		return nil, err
	}

	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fnerrors.InvocationError("nscloud", "unexpected status code %d", response.StatusCode)
	}

	keysData, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	parts := strings.Split(string(keysData), "\n")

	var keys []string
	for _, p := range parts {
		x := strings.TrimSpace(p)
		if x != "" {
			keys = append(keys, x)
		}
	}

	return keys, nil
}

func UserSSHKeys() (compute.Computable[[]string], error) {
	user, err := fnapi.LoadUser()
	if err != nil {
		return nil, err
	}

	if user.Clerk != nil {
		if user.Clerk.GithubUsername != "" {
			return deferFetchSSHKeys(user.Clerk.GithubUsername), nil
		}

		return nil, nil
	}

	if strings.HasSuffix(user.Username, "[bot]") {
		return nil, nil
	}

	return deferFetchSSHKeys(user.Username), nil
}

func deferFetchSSHKeys(username string) compute.Computable[[]string] {
	return compute.Inline(tasks.Action("github.fetch-public-ssh-keys").Arg("username", username), func(ctx context.Context) ([]string, error) {
		return fetchGithubSshKeys(ctx, username)
	})
}
